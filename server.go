package main

import (
	"html/template"
	"net/http"
	"path"

	cfg "workspace/config"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)


func loginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", nil)
}

func logoutAct(c *gin.Context) {
	cfg.Logout(c)
	c.HTML(http.StatusOK, "login.html", nil)
}

func loginAct(c *gin.Context) {

	if cfg.Auth(c) {
		c.HTML(http.StatusOK, "main.html", nil)
	} else {
		c.String(http.StatusOK, "登录失败")
	}
}

func uploadPage(c *gin.Context) {
	c.HTML(http.StatusOK, "upload.html", nil)
}

func uploadAct(c *gin.Context) {
	//从请求中读取文件
	f, err := c.FormFile("f1") //和从请求中获取携带的参数一样
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
	} else {
		//将读取到的文件保存到本地(服务端)
		dst := path.Join(cfg.GetUploadPath(), f.Filename)
		_ = c.SaveUploadedFile(f, dst)
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	}
}

func middleWare(r *gin.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path != "/s/login" && c.Request.URL.Path != "/fs" {
			uInfo := cfg.GetUser(c)
			if uInfo == nil {
				c.HTML(http.StatusOK, "login.html", nil)
				return
			}
		}
		c.Next()
	}
}

func runSession(server *gin.Engine) {

	sessionGroup := server.Group("/s")

	sessionGroup.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main.html", nil)
	})

	sessionGroup.GET("/login", loginPage)
	sessionGroup.POST("/login", loginAct)
	sessionGroup.GET("/logout", logoutAct)

	if cfg.GetUploadPath() != "" {
		sessionGroup.GET("/upload", uploadPage)
		sessionGroup.POST("/upload", uploadAct)
	}
}

func main() {
	cfg.Init("./config.json")
	server := gin.Default()
	store := cookie.NewStore([]byte("secret")) // 设置生成sessionId的密钥
	// mysession是返回給前端的sessionId名
	server.Use(sessions.Sessions("mysession", store))

	server.Static("/assets", "./template/assets")
	server.LoadHTMLGlob("template/html/*")
	server.Use(middleWare(server)) // 注册一个全局中间件

	server.SetFuncMap(template.FuncMap{
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
	})

	cfg.InitFileServers(server)
	server.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main.html", nil)
	})
	runSession(server)
	server.Run(":80")
}
