package web

import (
	"net/http"

	"github.com/doylecnn/new-nsfc-bot/web/middleware"
	"github.com/gin-gonic/gin"
)

// Index show index page. use templates/index.html
func (w Web) Index(c *gin.Context) {
	if v, exists := c.Get("authed"); exists {
		if authed, ok := v.(bool); ok && authed {
			authData, _ := c.Cookie("auth_data_str")
			userID, err := middleware.GetAuthDataInfo(authData, "id")
			if err != nil {
				_logger.Print(err)
				c.HTML(http.StatusOK, "index.html", nil)
				return
			}
			c.Redirect(http.StatusTemporaryRedirect, "/user/"+userID)
		}
	}
	c.HTML(http.StatusOK, "index.html", nil)
}
