package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lixiangling-com/pulsewatch/server/internal/platform/httpx"
)

const refreshCookieName = "refresh_token"

type Handler struct {
	service           *Service
	developmentCookie bool
}

func NewHandler(service *Service, developmentCookie bool) *Handler {
	return &Handler{service: service, developmentCookie: developmentCookie}
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type sessionResponse struct {
	AccessToken string       `json:"access_token"`
	ExpiresIn   int          `json:"expires_in"`
	User        userResponse `json:"user"`
}

func (h *Handler) Register(c *gin.Context) {
	request, ok := decodeCredentials(c)
	if !ok {
		return
	}
	session, err := h.service.Register(c.Request.Context(), request.Email, request.Password)
	if err != nil {
		h.writeRegisterError(c, err)
		return
	}
	h.writeSession(c, http.StatusCreated, session)
}

func (h *Handler) Login(c *gin.Context) {
	request, ok := decodeCredentials(c)
	if !ok {
		return
	}
	session, err := h.service.Login(c.Request.Context(), request.Email, request.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrInvalidPassword) {
			httpx.WriteError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "邮箱或密码不正确", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用", nil)
		return
	}
	h.writeSession(c, http.StatusOK, session)
}

func (h *Handler) Refresh(c *gin.Context) {
	raw, _ := c.Cookie(refreshCookieName)
	session, err := h.service.Refresh(c.Request.Context(), raw)
	if err != nil {
		h.clearRefreshCookie(c)
		if errors.Is(err, ErrUnauthorized) {
			httpx.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "认证已失效，请重新登录", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用", nil)
		return
	}
	h.writeSession(c, http.StatusOK, session)
}

func (h *Handler) Logout(c *gin.Context) {
	raw, _ := c.Cookie(refreshCookieName)
	err := h.service.Logout(c.Request.Context(), raw)
	h.clearRefreshCookie(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			httpx.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "认证已失效，请重新登录", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用", nil)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Me(c *gin.Context) {
	userID, ok := UserIDFromContext(c)
	if !ok {
		httpx.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "认证已失效，请重新登录", nil)
		return
	}
	user, err := h.service.CurrentUser(c.Request.Context(), [16]byte(userID))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			httpx.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "认证已失效，请重新登录", nil)
			return
		}
		httpx.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用", nil)
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}

func decodeCredentials(c *gin.Context) (credentialsRequest, bool) {
	var request credentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请检查输入", map[string][]string{})
		return credentialsRequest{}, false
	}
	return request, true
}

func (h *Handler) writeRegisterError(c *gin.Context, err error) {
	if errors.Is(err, ErrEmailAlreadyRegistered) {
		httpx.WriteError(c, http.StatusConflict, "EMAIL_ALREADY_REGISTERED", "该邮箱已注册", map[string][]string{"email": {"该邮箱已注册"}})
		return
	}
	if errors.Is(err, ErrInvalidPassword) {
		httpx.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请检查输入", map[string][]string{"password": {"密码长度应为 8 到 72 个字节"}})
		return
	}
	if errors.Is(err, ErrInvalidCredentials) {
		httpx.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请检查输入", map[string][]string{"email": {"请输入有效邮箱"}})
		return
	}
	httpx.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用", nil)
}

func (h *Handler) writeSession(c *gin.Context, status int, session Session) {
	h.setRefreshCookie(c, session.RefreshToken, time.Now().UTC().Add(RefreshTokenLifetime), int(RefreshTokenLifetime.Seconds()))
	c.JSON(status, sessionResponse{AccessToken: session.AccessToken, ExpiresIn: session.ExpiresIn, User: toUserResponse(session.User)})
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	h.setRefreshCookie(c, "", time.Unix(1, 0).UTC(), -1)
}

func (h *Handler) setRefreshCookie(c *gin.Context, value string, expires time.Time, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     "/api/v1/auth",
		Expires:  expires,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   !h.developmentCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func toUserResponse(user User) userResponse {
	return userResponse{ID: user.ID, Email: user.Email, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}
