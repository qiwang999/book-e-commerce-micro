package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/qiwang/book-e-commerce-micro/common/auth"
	"github.com/qiwang/book-e-commerce-micro/common/email"
	pb "github.com/qiwang/book-e-commerce-micro/proto/user"
	"github.com/qiwang/book-e-commerce-micro/service/user/model"
	"github.com/qiwang/book-e-commerce-micro/service/user/repository"
)

const (
	verifyCodeKeyPrefix = "verify:register:"
	verifyCodeTTL       = 5 * time.Minute
	verifyCodeLength    = 6
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type UserHandler struct {
	repo        *repository.UserRepository
	jwtManager  *auth.JWTManager
	rdb         *redis.Client
	emailSender *email.Sender
}

func NewUserHandler(repo *repository.UserRepository, jwtManager *auth.JWTManager, rdb *redis.Client, emailSender *email.Sender) *UserHandler {
	return &UserHandler{
		repo:        repo,
		jwtManager:  jwtManager,
		rdb:         rdb,
		emailSender: emailSender,
	}
}

func generateCode() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

func (h *UserHandler) SendVerificationCode(ctx context.Context, req *pb.SendCodeRequest, rsp *pb.CommonResponse) error {
	emailAddr := strings.TrimSpace(req.Email)
	if emailAddr == "" {
		rsp.Code = 400
		rsp.Message = "email is required"
		return nil
	}
	if !emailRegex.MatchString(emailAddr) {
		rsp.Code = 400
		rsp.Message = "invalid email format"
		return nil
	}

	_, err := h.repo.GetUserByEmail(emailAddr)
	if err == nil {
		rsp.Code = 400
		rsp.Message = "email already registered"
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		rsp.Code = 500
		rsp.Message = "failed to check email"
		return nil
	}

	key := verifyCodeKeyPrefix + emailAddr
	ttl, _ := h.rdb.TTL(ctx, key).Result()
	if ttl > verifyCodeTTL-time.Minute {
		rsp.Code = 429
		rsp.Message = "verification code already sent, please try again later"
		return nil
	}

	code := generateCode()
	if err := h.rdb.Set(ctx, key, code, verifyCodeTTL).Err(); err != nil {
		rsp.Code = 500
		rsp.Message = "failed to store verification code"
		return nil
	}

	go func() {
		if err := h.emailSender.SendVerificationCode(emailAddr, code); err != nil {
			log.Printf("[user] failed to send verification email to %s: %v", emailAddr, err)
		}
	}()

	rsp.Code = 200
	rsp.Message = "verification code sent"
	return nil
}

func (h *UserHandler) Register(ctx context.Context, req *pb.RegisterRequest, rsp *pb.AuthResponse) error {
	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)

	if req.Email == "" || req.Password == "" || req.Name == "" {
		rsp.Code = 400
		rsp.Message = "email, password and name are required"
		return nil
	}
	if req.Code == "" {
		rsp.Code = 400
		rsp.Message = "verification code is required"
		return nil
	}
	if !emailRegex.MatchString(req.Email) {
		rsp.Code = 400
		rsp.Message = "invalid email format"
		return nil
	}
	if len(req.Password) < 6 {
		rsp.Code = 400
		rsp.Message = "password must be at least 6 characters"
		return nil
	}

	key := verifyCodeKeyPrefix + req.Email
	storedCode, err := h.rdb.Get(ctx, key).Result()
	if err != nil {
		rsp.Code = 400
		rsp.Message = "verification code expired or not sent"
		return nil
	}
	if storedCode != req.Code {
		rsp.Code = 400
		rsp.Message = "invalid verification code"
		return nil
	}
	h.rdb.Del(ctx, key)

	_, err = h.repo.GetUserByEmail(req.Email)
	if err == nil {
		rsp.Code = 400
		rsp.Message = "email already registered"
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		rsp.Code = 500
		rsp.Message = "failed to check email"
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		rsp.Code = 500
		rsp.Message = "failed to hash password"
		return nil
	}

	user := &model.User{
		Email:        req.Email,
		PasswordHash: string(hash),
		Name:         req.Name,
		Role:         "user",
		Status:       1,
	}
	if err := h.repo.CreateUser(user); err != nil {
		rsp.Code = 500
		rsp.Message = "failed to create user"
		return nil
	}

	profile := &model.UserProfile{
		UserID:             user.ID,
		FavoriteCategories: model.StringSlice{},
		FavoriteAuthors:    model.StringSlice{},
		ReadingPreferences: model.StringSlice{},
	}
	if err := h.repo.CreateProfile(profile); err != nil {
		rsp.Code = 500
		rsp.Message = "failed to create profile"
		return nil
	}

	token, err := h.jwtManager.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		rsp.Code = 500
		rsp.Message = "failed to generate token"
		return nil
	}

	rsp.Code = 200
	rsp.Message = "success"
	rsp.Token = token
	rsp.User = &pb.UserInfo{
		Id:    user.ID,
		Email: user.Email,
		Name:  user.Name,
		Role:  user.Role,
	}
	return nil
}

func (h *UserHandler) Login(ctx context.Context, req *pb.LoginRequest, rsp *pb.AuthResponse) error {
	if req.Email == "" || req.Password == "" {
		rsp.Code = 400
		rsp.Message = "email and password are required"
		return nil
	}

	user, err := h.repo.GetUserByEmail(req.Email)
	if err != nil {
		rsp.Code = 401
		rsp.Message = "invalid email or password"
		return nil
	}

	if user.Status != 1 {
		rsp.Code = 403
		rsp.Message = "account is disabled"
		return nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		rsp.Code = 401
		rsp.Message = "invalid email or password"
		return nil
	}

	token, err := h.jwtManager.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		rsp.Code = 500
		rsp.Message = "failed to generate token"
		return nil
	}

	rsp.Code = 200
	rsp.Message = "success"
	rsp.Token = token
	rsp.User = &pb.UserInfo{
		Id:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		AvatarUrl: user.AvatarURL,
		Role:      user.Role,
	}
	return nil
}

func (h *UserHandler) GetProfile(ctx context.Context, req *pb.GetProfileRequest, rsp *pb.UserProfile) error {
	user, err := h.repo.GetUserByID(req.UserId)
	if err != nil {
		return errors.New("user not found")
	}

	profile, err := h.repo.GetProfileByUserID(req.UserId)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("failed to get profile")
	}

	rsp.UserId = user.ID
	rsp.Email = user.Email
	rsp.Name = user.Name
	rsp.AvatarUrl = user.AvatarURL

	if profile != nil {
		rsp.Phone = profile.Phone
		rsp.Gender = profile.Gender
		if profile.Birthday != nil {
			rsp.Birthday = profile.Birthday.Format("2006-01-02")
		}
		rsp.FavoriteCategories = []string(profile.FavoriteCategories)
		rsp.FavoriteAuthors = []string(profile.FavoriteAuthors)
		rsp.ReadingPreferences = []string(profile.ReadingPreferences)
	}
	return nil
}

func (h *UserHandler) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest, rsp *pb.CommonResponse) error {
	user, err := h.repo.GetUserByID(req.UserId)
	if err != nil {
		rsp.Code = 404
		rsp.Message = "user not found"
		return nil
	}

	user.Name = req.Name
	user.AvatarURL = req.AvatarUrl
	if err := h.repo.UpdateUser(user); err != nil {
		rsp.Code = 500
		rsp.Message = "failed to update user"
		return nil
	}

	profile, err := h.repo.GetProfileByUserID(req.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			profile = &model.UserProfile{UserID: req.UserId}
		} else {
			rsp.Code = 500
			rsp.Message = "failed to get profile"
			return nil
		}
	}

	profile.Phone = req.Phone
	profile.Gender = req.Gender
	if req.Birthday == "" {
		profile.Birthday = nil
	} else {
		birthday, err := time.Parse("2006-01-02", req.Birthday)
		if err != nil {
			rsp.Code = 400
			rsp.Message = "invalid birthday format, expected YYYY-MM-DD"
			return nil
		}
		profile.Birthday = &birthday
	}

	if profile.ID == 0 {
		err = h.repo.CreateProfile(profile)
	} else {
		err = h.repo.UpdateProfile(profile)
	}
	if err != nil {
		rsp.Code = 500
		rsp.Message = "failed to update profile"
		return nil
	}

	rsp.Code = 200
	rsp.Message = "success"
	return nil
}

func (h *UserHandler) GetUserPreferences(ctx context.Context, req *pb.GetPreferencesRequest, rsp *pb.UserPreferences) error {
	profile, err := h.repo.GetProfileByUserID(req.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			rsp.UserId = req.UserId
			return nil
		}
		return errors.New("failed to get preferences")
	}

	rsp.UserId = req.UserId
	rsp.FavoriteCategories = []string(profile.FavoriteCategories)
	rsp.FavoriteAuthors = []string(profile.FavoriteAuthors)
	rsp.ReadingPreferences = []string(profile.ReadingPreferences)
	return nil
}

func (h *UserHandler) UpdateUserPreferences(ctx context.Context, req *pb.UpdatePreferencesRequest, rsp *pb.CommonResponse) error {
	profile, err := h.repo.GetProfileByUserID(req.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			profile = &model.UserProfile{UserID: req.UserId}
		} else {
			rsp.Code = 500
			rsp.Message = "failed to get profile"
			return nil
		}
	}

	profile.FavoriteCategories = model.StringSlice(req.FavoriteCategories)
	profile.FavoriteAuthors = model.StringSlice(req.FavoriteAuthors)
	profile.ReadingPreferences = model.StringSlice(req.ReadingPreferences)

	if profile.ID == 0 {
		err = h.repo.CreateProfile(profile)
	} else {
		err = h.repo.UpdateProfile(profile)
	}
	if err != nil {
		rsp.Code = 500
		rsp.Message = "failed to update preferences"
		return nil
	}

	rsp.Code = 200
	rsp.Message = "success"
	return nil
}

func (h *UserHandler) GetAddress(ctx context.Context, req *pb.GetAddressRequest, rsp *pb.Address) error {
	if req.AddressId == 0 {
		return errors.New("address_id is required")
	}

	addr, err := h.repo.GetAddressByID(req.AddressId)
	if err != nil {
		return errors.New("address not found")
	}

	rsp.Id = addr.ID
	rsp.UserId = addr.UserID
	rsp.Name = addr.Name
	rsp.Phone = addr.Phone
	rsp.Province = addr.Province
	rsp.City = addr.City
	rsp.District = addr.District
	rsp.Detail = addr.Detail
	rsp.IsDefault = addr.IsDefault
	return nil
}

func (h *UserHandler) ListAddresses(ctx context.Context, req *pb.ListAddressesRequest, rsp *pb.AddressListResponse) error {
	addresses, err := h.repo.ListAddressesByUserID(req.UserId)
	if err != nil {
		return errors.New("failed to list addresses")
	}

	rsp.Addresses = make([]*pb.Address, 0, len(addresses))
	for _, addr := range addresses {
		rsp.Addresses = append(rsp.Addresses, &pb.Address{
			Id:        addr.ID,
			UserId:    addr.UserID,
			Name:      addr.Name,
			Phone:     addr.Phone,
			Province:  addr.Province,
			City:      addr.City,
			District:  addr.District,
			Detail:    addr.Detail,
			IsDefault: addr.IsDefault,
		})
	}
	return nil
}

func (h *UserHandler) CreateAddress(ctx context.Context, req *pb.CreateAddressRequest, rsp *pb.Address) error {
	if req.UserId == 0 {
		return errors.New("user_id is required")
	}
	if req.Name == "" || req.Phone == "" || req.Province == "" || req.City == "" || req.Detail == "" {
		return errors.New("name, phone, province, city and detail are required")
	}

	addr := &model.UserAddress{
		UserID:    req.UserId,
		Name:      req.Name,
		Phone:     req.Phone,
		Province:  req.Province,
		City:      req.City,
		District:  req.District,
		Detail:    req.Detail,
		IsDefault: req.IsDefault,
	}

	if err := h.repo.CreateAddress(addr); err != nil {
		return errors.New("failed to create address")
	}

	rsp.Id = addr.ID
	rsp.UserId = addr.UserID
	rsp.Name = addr.Name
	rsp.Phone = addr.Phone
	rsp.Province = addr.Province
	rsp.City = addr.City
	rsp.District = addr.District
	rsp.Detail = addr.Detail
	rsp.IsDefault = addr.IsDefault
	return nil
}

func (h *UserHandler) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest, rsp *pb.ValidateTokenResponse) error {
	claims, err := h.jwtManager.ValidateToken(req.Token)
	if err != nil {
		rsp.Valid = false
		return nil
	}

	user, err := h.repo.GetUserByID(claims.UserID)
	if err != nil || user.Status != 1 {
		rsp.Valid = false
		return nil
	}

	rsp.Valid = true
	rsp.UserId = claims.UserID
	rsp.Role = claims.Role
	return nil
}
