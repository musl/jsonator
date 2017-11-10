package main

import (
	"flag"
	"fmt"
	"github.com/fvbock/endless"
	"github.com/gin-gonic/gin"
	"github.com/orcaman/concurrent-map"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"syscall"
)

type Document interface{}

type Config struct {
	LogPath  string
	PidPath  string
	BindAddr string
}

type Store struct {
	cmap.ConcurrentMap
}

func GetStatus(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

func GetAll(c *gin.Context) {
	store := c.MustGet("store").(*Store)

	c.JSON(http.StatusOK, store.Items())
}

func GetCount(c *gin.Context) {
	store := c.MustGet("store").(*Store)

	c.JSON(http.StatusOK, store.Count())
}

func GetKeys(c *gin.Context) {
	store := c.MustGet("store").(*Store)

	c.JSON(http.StatusOK, store.Keys())
}

func GetDoc(c *gin.Context) {
	store := c.MustGet("store").(*Store)
	key := c.Param("key")
	value, _ := store.Get(key)

	c.JSON(http.StatusOK, value)
}

func PutDoc(c *gin.Context) {
	var doc Document
	store := c.MustGet("store").(*Store)
	key := c.Param("key")

	err := c.ShouldBindJSON(&doc)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, nil)
		return
	}

	store.Set(key, doc)
	c.JSON(http.StatusOK, nil)
}

func DeleteDoc(c *gin.Context) {
	store := c.MustGet("store").(*Store)
	key := c.Param("key")

	store.Remove(key)

	c.JSON(http.StatusOK, nil)
}

func StoreMiddleware(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("store", store)
		c.Next()
	}
}

func main() {
	config := Config{}

	flag.StringVar(&config.BindAddr, "bind", ":8080", "bind address and port")
	flag.StringVar(&config.LogPath, "log", "./log", "path to log file")
	flag.StringVar(&config.PidPath, "pid", "./pid", "path to pid file")
	flag.Parse()

	logfile, _ := os.Create(config.LogPath)
	gin.DefaultWriter = io.MultiWriter(logfile)
	log.SetOutput(gin.DefaultWriter)

	dev_null, _ := os.Open("/dev/null")
	syscall.Dup2(int(dev_null.Fd()), 1)
	syscall.Dup2(int(dev_null.Fd()), 2)

	router := gin.Default()

	store := Store{cmap.New()}
	router.Use(StoreMiddleware(&store))

	router.GET("/status", GetStatus)
	router.GET("/count", GetCount)
	router.GET("/keys", GetKeys)
	router.GET("/doc", GetAll)
	router.GET("/doc/:key", GetDoc)
	router.PUT("/doc/:key", PutDoc)
	router.DELETE("/doc/:key", DeleteDoc)

	/* do we need this hook, pid file? */
	srv := endless.NewServer(config.BindAddr, router)
	srv.BeforeBegin = func(add string) {
		pid := fmt.Sprintf("%d\n", syscall.Getpid())
		log.Printf("Pid is now: %s", pid)
		ioutil.WriteFile(config.PidPath, []byte(pid), 0644)
	}

	err := srv.ListenAndServe()
	log.Printf(err.Error())
	panic(err)
}
