package repository

import (
	"github.com/qiwang/book-e-commerce-micro/service/user/model"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) CreateUser(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepository) GetUserByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetUserByID(id uint64) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) UpdateUser(user *model.User) error {
	return r.db.Model(user).Updates(user).Error
}

func (r *UserRepository) CreateProfile(profile *model.UserProfile) error {
	return r.db.Create(profile).Error
}

func (r *UserRepository) GetProfileByUserID(userID uint64) (*model.UserProfile, error) {
	var profile model.UserProfile
	err := r.db.Where("user_id = ?", userID).First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *UserRepository) UpdateProfile(profile *model.UserProfile) error {
	return r.db.Save(profile).Error
}

func (r *UserRepository) GetAddressByID(id uint64) (*model.UserAddress, error) {
	var addr model.UserAddress
	err := r.db.First(&addr, id).Error
	if err != nil {
		return nil, err
	}
	return &addr, nil
}

func (r *UserRepository) ListAddressesByUserID(userID uint64) ([]model.UserAddress, error) {
	var addresses []model.UserAddress
	err := r.db.Where("user_id = ?", userID).Order("is_default DESC, id DESC").Find(&addresses).Error
	if err != nil {
		return nil, err
	}
	return addresses, nil
}

func (r *UserRepository) CreateAddress(addr *model.UserAddress) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if addr.IsDefault {
			if err := tx.Model(&model.UserAddress{}).
				Where("user_id = ? AND is_default = ?", addr.UserID, true).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(addr).Error
	})
}

func (r *UserRepository) UpdateAddress(addr *model.UserAddress) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if addr.IsDefault {
			if err := tx.Model(&model.UserAddress{}).
				Where("user_id = ? AND is_default = ? AND id != ?", addr.UserID, true, addr.ID).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(addr).Error
	})
}

func (r *UserRepository) DeleteAddress(addressID, userID uint64) error {
	result := r.db.Where("id = ? AND user_id = ?", addressID, userID).Delete(&model.UserAddress{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

