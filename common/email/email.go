package email

import (
	"fmt"

	"github.com/qiwang/book-e-commerce-micro/common/config"
	"gopkg.in/gomail.v2"
)

type Sender struct {
	dialer *gomail.Dialer
	from   string
}

func NewSender(cfg config.EmailConfig) *Sender {
	d := gomail.NewDialer(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	return &Sender{dialer: d, from: cfg.From}
}

func (s *Sender) SendVerificationCode(to, code string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", s.from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", "BookHive 注册验证码")
	m.SetBody("text/html", fmt.Sprintf(`
		<div style="max-width:480px;margin:0 auto;font-family:sans-serif;padding:24px;">
			<h2 style="color:#1a73e8;">BookHive 邮箱验证</h2>
			<p>您好，您正在注册 BookHive 账号，验证码为：</p>
			<div style="font-size:32px;font-weight:bold;letter-spacing:8px;color:#1a73e8;
				background:#f0f4ff;padding:16px 24px;border-radius:8px;text-align:center;margin:16px 0;">
				%s
			</div>
			<p style="color:#666;font-size:14px;">验证码 5 分钟内有效，请勿泄露给他人。</p>
			<p style="color:#999;font-size:12px;margin-top:24px;">如非本人操作，请忽略此邮件。</p>
		</div>
	`, code))

	return s.dialer.DialAndSend(m)
}
