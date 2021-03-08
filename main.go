package main

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/go-yaml/yaml"
)

type Permission struct {
	All   []string
	Read  []string
	Write []string
	List  []string
}

type Config struct {
	Tokens      map[string]string
	Permissions map[string]Permission
}

func configData() ([]byte, error) {
	res, err := ioutil.ReadFile("config.yml")
	if err == nil {
		return res, nil
	} else if os.IsNotExist(err) { // from env
		str := os.Getenv("SAKVAS_CONFIG")
		return []byte(str), nil
	} else {
		return nil, err
	}
}

var config = Config{}

func resolveToken(c *gin.Context) (string, error) {
	authorizeHeader := c.Request.Header.Get("Authorization")
	if authorizeHeader == "" {
		return "anonymous", nil
	}
	if !strings.HasPrefix(authorizeHeader, "Bearer ") {
		c.Abort()
		c.String(401, "invalid authorize header")
		return "", c.Error(errors.New("invalid authorize header"))
	}
	token := authorizeHeader[len("Bearer "):]
	for key, tok := range config.Tokens {
		if tok == token {
			return key, nil
		}
	}
	c.Abort()
	c.String(401, "invalid token")
	return "", c.Error(errors.New("invalid token"))
}

func contains(array *[]string, search string) bool {
	for _, str := range *array {
		if str == search {
			return true
		}
	}
	return false
}

type PermissionType uint8

const (
	Read PermissionType = iota
	Write
	List
)

func isAllowed(c *gin.Context, path string, ptype PermissionType) error {
	token, err := resolveToken(c)
	if err != nil {
		return err
	}
	for prefix, permission := range config.Permissions {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		if contains(&permission.All, token) {
			return nil
		}
		if ptype == Read && contains(&permission.Read, token) {
			return nil
		} else if ptype == Write && contains(&permission.Write, token) {
			return nil
		} else if ptype == List && contains(&permission.List, token) {
			return nil
		}
	}
	c.Abort()
	c.String(403, "you don't have enough permission needed by this prefix")
	return c.Error(errors.New("you don't have enough permission needed by this prefix"))
}

func main() {
	configData, err := configData()
	if err != nil {
		panic(err)
	}
	if err := yaml.Unmarshal(configData, &config); err != nil {
		panic(err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		DB:       0,
		Password: "",
	})
	systemPrefix := os.Getenv("SAKVAS_PREFIX")
	if systemPrefix == "" {
		systemPrefix = "sakvas"
	}

	router := gin.Default()
	router.GET("/v1/key/*key", func(c *gin.Context) {
		path := c.Param("key")
		if err := isAllowed(c, path, Read); err != nil {
			panic(err)
		}
		res, err := rdb.Get(c, systemPrefix+":value"+path).Bytes()
		if err == redis.Nil {
			c.Abort()
			c.String(404, "this key doesn't found")
			return
		} else if err != nil {
			panic(err)
		}
		c.Status(200)
		c.Data(200, "application/octet-stream", res)
	})
	router.PUT("/v1/key/*key", func(c *gin.Context) {
		path := c.Param("key")
		if err := isAllowed(c, path, Write); err != nil {
			panic(err)
		}
		rawData, err := c.GetRawData()
		if err != nil {
			panic(err)
		}
		if err := rdb.Set(c, systemPrefix+":value"+path, rawData, 0).Err(); err != nil {
			panic(err)
		}
		c.Status(204)
	})
	router.GET("/v1/list/*prefix", func(c *gin.Context) {
		path := c.Param("prefix")
		if err := isAllowed(c, path, List); err != nil {
			panic(err)
		}
		shouldRemovePrefix := systemPrefix + ":value"
		keys, err := rdb.Keys(c, systemPrefix+":value"+path+"*").Result()
		if err != nil {
			panic(err)
		}
		for i, key := range keys {
			keys[i] = key[len(shouldRemovePrefix):]
		}
		c.JSON(200, keys)
	})
	router.Run(":3000")
}
