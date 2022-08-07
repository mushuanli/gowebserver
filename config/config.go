package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type amiConfig struct {
	amiCmd	string 	
	amiServers	map[string] string
}

type config struct {
	auth           	map[string]string
	userList       	map[string] User
	downloadServer 	[]string
	uploadPath     	string
	amiCtl			*amiConfig
}

var cfgInfo config

func print_map(m map[string]interface{}) {
	for k, v := range m {
		switch value := v.(type) {
		case nil:
			fmt.Println(k, "is nil", "null")
		case string:
			fmt.Println(k, "is string", value)
		case int:
			fmt.Println(k, "is int", value)
		case float64:
			fmt.Println(k, "is float64", value)
		case []interface{}:
			fmt.Println(k, "is an array:")
			for i, u := range value {
				fmt.Println(i, u)
			}
		case map[string]interface{}:
			fmt.Println(k, "is an map:")
			print_map(value)
		default:
			fmt.Println(k, "is unknown type", fmt.Sprintf("%T", v))
		}
	}
}

func initLog(filename string) error {
	dir := filepath.Dir(filename)
	if dir == "" {
		dir = "."
	}

	file := dir + "/message" + ".txt"
	logFile, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0766)
	if err != nil {
		return err
	}
	log.SetOutput(logFile) // 将文件设置为log输出的文件
	log.SetPrefix("[]")
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.LUTC)
	gin.DefaultWriter = io.MultiWriter(logFile, os.Stdout)
	return nil
}

func initAmi(amiPath string ) *amiConfig{
	if amiPath == "" {
		return nil
	}

	iniFile   := path.Join(amiPath,"config.ini")
	file, err := os.Open(iniFile);
	if  err != nil {
		log.Println(err);
		return nil
	}

	defer file.Close()

	amiCtl 					:= new(amiConfig)
	amiCtl.amiCmd			= path.Join(amiPath,"excli.sh")
	amiCtl.amiServers     	= make(map[string]string)

	presize	:= len("@esxi_info")+1
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line,"<@esxi_info"){
			continue
		}

		s:=strings.SplitN(line[presize:],"@>=",2)
		if s[0] != "" {
			if s[0][0] == '.' {
				s[0] = s[0][1:]
			}
		}

		if s[1] != "" {
			v:=strings.SplitN(s[1],";",2)
			s[1]	= v[0]
			if s[1] != "" {
				k:=strings.SplitN(s[1],"@",2)
				if len(k) > 1 {
					s[1]=k[1]
				}
			}
		}
		
		amiCtl.amiServers[s[1]]	= s[0]
	}
 
	return amiCtl
}

func Init(filename string) bool {
	err := initLog(filename)
	if err != nil {
		fmt.Println(err)
		return false
	}
	// Open our jsonFile
	jsonFile, err := os.Open(filename)
	// if we os.Open returns an error then handle it
	if err != nil {
		log.Println(err)
		return false
	}

	fmt.Printf("Successfully Opened config file: %s\n", filename)
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	cfgInfo.auth     = make(map[string]string)
	cfgInfo.userList = make(map[string] User)

	// read our opened xmlFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)

	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'users' which we defined above

	var m map[string]interface{}
	m = make(map[string]interface{}) //必可不少，分配内存

	json.Unmarshal(byteValue, &m)
	auth := m["auth"].(map[string]interface{})
	if auth == nil {
		log.Println("no auth info")
		return false
	}
	for k, v := range auth {
		var name = k
		var value = v.(string)
		cfgInfo.auth[name] = value
	}

	downloadServer := m["downloadServer"].([]interface{})
	if downloadServer != nil {
		for v := range downloadServer {
			var value = downloadServer[v].(string)
			cfgInfo.downloadServer = append(cfgInfo.downloadServer, value)
		}
	}

	cfgInfo.uploadPath = m["uploadPath"].(string)
	if cfgInfo.uploadPath != "" {
		cfgInfo.uploadPath += "/"
	}

	amiPath := m["amipath"]
	if amiPath != nil {
		cfgInfo.amiCtl	= initAmi(m["amipath"].(string))
	}
	return true
}


func InitFileServers(server *gin.Engine) {
	svrnum := len(cfgInfo.downloadServer)
	if svrnum > 0 {
		server.StaticFS("/fs", gin.Dir(cfgInfo.downloadServer[0], true))
		for i := 1; i < svrnum; i++ {
			url := fmt.Sprintf("/fs%d", i)
			server.StaticFS(url, gin.Dir(cfgInfo.downloadServer[0], true))
		}
	}
}

func GetUploadPath() string {
	return cfgInfo.uploadPath
}


type User struct {
	// binding:"required"修饰的字段，若接收为空值，则报错，是必须字段
	Username string `form:"username" json:"user" uri:"user" xml:"user" binding:"required"`
	Password string `form:"password" json:"password" uri:"password" xml:"password" binding:"required"`
	IPAddr   string
}

func Auth(c *gin.Context) bool {
	var loginVo User
	if err := c.ShouldBind(&loginVo); err != nil {
		return false
	}

	if cfgInfo.auth[loginVo.Username] != loginVo.Password {
		return false
	}

	loginVo.IPAddr = c.Request.RemoteAddr[:strings.IndexByte(c.Request.RemoteAddr, ':')]
	uuid := uuid.New().String()
	session := sessions.Default(c)
	session.Set("currentUser", loginVo.Username)
	session.Set("currentID", uuid)
	cfgInfo.userList[uuid] = loginVo

	// 一定要Save否则不生效，若未使用gob注册User结构体，调用Save时会返回一个Error
	session.Save()
	log.Printf("<%s:%s> LOGIN\n", loginVo.Username, loginVo.IPAddr)

	return true
}

func GetUser(c *gin.Context) *User {
	session := sessions.Default(c)
	user := session.Get("currentUser")
	uuid := session.Get("currentID")
	if user == nil || uuid == nil {
		return nil
	}
	uname := user.(string)
	uid := uuid.(string)
	if uInfo, ok := cfgInfo.userList[uid] ; ok  {
		if  uInfo.Username == uname {
			return &uInfo
		}
	}

	return nil
}

func Logout(c *gin.Context) bool {
	session := sessions.Default(c)
	uuid := session.Get("currentID")
	if uuid != nil {
		uid := uuid.(string)
		delete(cfgInfo.userList,uid)
	}
	session.Delete("currentUser")
	session.Delete("currentID")

	session.Save()
	return true
}