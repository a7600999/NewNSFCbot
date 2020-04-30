package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

var _logger zerolog.Logger

//TelegramAuth 使用telegram 来登录
func TelegramAuth(secretKey [32]byte, logger zerolog.Logger) gin.HandlerFunc {
	_logger = logger
	return func(c *gin.Context) {
		authData, err := c.Cookie("auth_data_str")
		if err != nil || len(authData) == 0 {
			redirectToLogin(c, "cookie: auth_data_str is not set")
			return
		}
		expectedHash, err := c.Cookie("auth_data_hash")
		if err != nil || len(expectedHash) == 0 {
			redirectToLogin(c, "cookie: auth_data_hash is not set")
			return
		}

		info, err := GetAuthDataInfo(authData, "auth_date")
		if err != nil {
			redirectToLogin(c, err.Error())
			return
		}
		authDate, err := strconv.Atoi(info)
		if err != nil {
			redirectToLogin(c, fmt.Sprintf("authdate:%s, err: %v", info, err))
			return
		}

		mac := hmac.New(sha256.New, secretKey[:])
		io.WriteString(mac, authData)
		hash := fmt.Sprintf("%x", mac.Sum(nil))
		if expectedHash != hash {
			redirectToLogin(c, "data is not from Telegram")
			return
		} else if int64(time.Now().Sub(time.Unix(int64(authDate), 0)).Seconds()) > 86400 {
			redirectToLogin(c, "Data is outdated")
			return
		}
		c.Set("authed", true)
		c.Next()
	}
}

// TelegramAdminAuth 确定是不是admin
func TelegramAdminAuth(secretKey [32]byte, adminUID int) gin.HandlerFunc {
	return func(c *gin.Context) {
		authData, err := c.Cookie("auth_data_str")
		if err != nil || len(authData) == 0 {
			redirectToLogin(c, "cookie: auth_data_str is not set")
			return
		}
		expectedHash, err := c.Cookie("auth_data_hash")
		if err != nil || len(expectedHash) == 0 {
			redirectToLogin(c, "cookie: auth_data_hash is not set")
			return
		}
		if uid, err := GetAuthDataInfo(authData, "id"); err == nil {
			if id, err := strconv.ParseInt(uid, 10, 64); err == nil && int(id) == adminUID {
				info, err := GetAuthDataInfo(authData, "auth_date")
				if err != nil {
					redirectToLogin(c, err.Error())
					return
				}
				authDate, err := strconv.Atoi(info)
				if err != nil {
					redirectToLogin(c, fmt.Sprintf("authdate:%s, err: %v", info, err))
					return
				}

				mac := hmac.New(sha256.New, secretKey[:])
				io.WriteString(mac, authData)
				hash := fmt.Sprintf("%x", mac.Sum(nil))
				if expectedHash != hash {
					redirectToLogin(c, "data is not from Telegram")
					return
				} else if int64(time.Now().Sub(time.Unix(int64(authDate), 0)).Seconds()) > 86400 {
					redirectToLogin(c, "Data is outdated")
					return
				}

				c.Set("admin_authed", true)
				c.Next()
				return
			}
		}
		_logger.Error().Err(err).Msg("auth failed")
		redirectToLogin(c, "auth failed")
		return
	}
}

func redirectToLogin(c *gin.Context, errorMessage string) {
	DelCookie(c, "auth_data_str")
	DelCookie(c, "auth_data_hash")
	DelCookie(c, "admin_authed")
	DelCookie(c, "authed")
	_logger.Printf("errormessage:%s", errorMessage)
	c.Redirect(http.StatusTemporaryRedirect, "/login?error=LoginFailed")
	c.Abort()
}

// GetAuthDataInfo get user auth data info
func GetAuthDataInfo(authData, key string) (value string, err error) {
	err = fmt.Errorf("key: %s not found", key)
	s := key + "="
	lIdx := strings.Index(authData, s)
	if lIdx == -1 {
		return
	}
	lIdx += len(s)
	rIdx := strings.Index(authData[lIdx:], "\n")
	if rIdx == -1 {
		return
	}
	rIdx += lIdx
	value = strings.TrimSpace(authData[lIdx:rIdx])
	if len(value) == 0 {
		err = fmt.Errorf("key: %s is exists, but value is empty", key)
	} else {
		err = nil
	}
	return
}

// DelCookie delete cookie
func DelCookie(c *gin.Context, name string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		Domain:   c.Request.URL.Host,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}
