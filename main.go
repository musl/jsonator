package main

import (
	//"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
	"github.com/newrelic/go-agent"
	"github.com/orcaman/concurrent-map"
	"io"
	"log"
	"net/http"
	"os"
	"syscall"
)

// A generic type to hold arbitrary JSON.
type Document interface{}

/*
 * A structure to hold all of the values given on the command line as
 * flags.
 */
type Config struct {
	LogPath      string `default:"/dev/stdout"`
	BindAddr     string `default:":8080"`
	NewRelicKey  string `required:"false"`
	NewRelicName string `default:"jsonator"`
}

/*
 * App statistics.
 */
type Stats struct {
	DocumentCount int    `json:"document_count"`
	DocumentBytes uint64 `json:"document_bytes"`
}

/*
 * Wrap a concurrent map implementation in a type so that it's easy to
 * replace or plug different implementations in later.
 */
type Store struct {
	cmap.ConcurrentMap
}

/*
 * Create a new store to further hide the complexity of having an
 * interchangable map implementation.
 */
func NewStore() Store {
	return Store{cmap.New()}
}

/*
 * Serve a simple status endpoint for automation and basic health
 * monitoring - say during deployment.
 */
func GetStatus(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

/*
 * Serve app statistics
 */
func GetStats(c *gin.Context) {
	store := c.MustGet("store").(*Store)
	stats := Stats{
		DocumentCount: store.Count(),
		DocumentBytes: 0,
	}

	c.JSON(http.StatusOK, stats)
}

/*
 * Easy backup for small-to medium datasets.
 */
func GetAll(c *gin.Context) {
	store := c.MustGet("store").(*Store)

	c.JSON(http.StatusOK, store.Items())
}

/*
 * Provide the number of documents stored in memory.
 */
func GetCount(c *gin.Context) {
	store := c.MustGet("store").(*Store)

	c.JSON(http.StatusOK, store.Count())
}

/*
 * Provide the list of all document keys.
 */
func GetKeys(c *gin.Context) {
	store := c.MustGet("store").(*Store)

	c.JSON(http.StatusOK, store.Keys())
}

/*
 * Retrieve a document.
 */
func GetDoc(c *gin.Context) {
	store := c.MustGet("store").(*Store)
	key := c.Param("key")
	value, _ := store.Get(key)

	c.JSON(http.StatusOK, value)
}

/*
 * Store arbitrary but well-formed JSON documents in memory.
 */
func PutDoc(c *gin.Context) {
	var doc Document
	store := c.MustGet("store").(*Store)
	key := c.Param("key")

	err := c.ShouldBindJSON(&doc)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, "")
		return
	}

	store.Set(key, doc)
	c.JSON(http.StatusNoContent, nil)
}

/*
 * Store arbitrary but well-formed collections of JSON documents in
 * memory.
 */
func PutBatch(c *gin.Context) {
	var batch map[string]interface{}
	store := c.MustGet("store").(*Store)

	err := c.ShouldBindJSON(&batch)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, "")
		return
	}

	for k, v := range batch {
		store.Set(k, v)
	}

	c.JSON(http.StatusNoContent, nil)
}

/*
 * Allow removal of documents.
 */
func DeleteDoc(c *gin.Context) {
	store := c.MustGet("store").(*Store)
	key := c.Param("key")

	store.Remove(key)

	c.JSON(http.StatusNoContent, nil)
}

/*
 * We want to be able to pass a reference to our thread-safe document
 * store into request handlers so that requests can mutate that store.
 * To do this, we need to create a gin.HandlerFunc that sets that
 * reference before calling the next handler in the chain. This function
 * returns another function so that it can be passed as an argument to
 * a gin.Router, while closing over the reference to our store.
 */
func StoreMiddleware(key string, store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(key, store)
		c.Next()
	}
}

/*
 * Use NewRelic to track statistics.
 */
func NewRelicMiddleware(name, key string) gin.HandlerFunc {
	config := newrelic.NewConfig(name, key)
	app, err := newrelic.NewApplication(config)
	if err != nil {
		panic(err.Error())
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		txn_name := fmt.Sprintf("%s", path)

		txn := app.StartTransaction(txn_name, c.Writer, c.Request)
		defer txn.End()

		c.Next()
	}
}

/*
 * We've got to have a main entry point.
 */
func main() {

	// Handle command-line flags.
	// TODO: extend this to support environment variables as well.
	config := Config{}
	err := envconfig.Process("jsonator", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	/*
		flag.StringVar(&config.BindAddr, "bind", ":8080", "bind address and port")
		flag.StringVar(&config.LogPath, "log", "/dev/stdout", "path to log file")
		flag.StringVar(&config.NewRelicKey, "newrelic-key", "", "NewRelic license key")
		flag.StringVar(&config.NewRelicName, "newrelic-name", "jsonator", "NewRelic app name")
		flag.Parse()
	*/

	// Configure logging to make debugging easy and provide a single,
	// consistent way to get log info out of the app.
	logfile, _ := os.Create(config.LogPath)
	gin.DefaultWriter = io.MultiWriter(logfile)
	log.SetOutput(gin.DefaultWriter)

	// Redirect stderr to stdout for container happieness
	dev_null, _ := os.Open("/dev/stdout")
	syscall.Dup2(int(dev_null.Fd()), 2)

	// Get a new, bells-and-whistles-included gin.Router.
	router := gin.Default()

	// Use our middleware to make our shared state availible to request
	// handlers.
	store := NewStore()
	router.Use(StoreMiddleware("store", &store))

	// Use NewRelic middleware for request stats.
	if config.NewRelicKey != "" && config.NewRelicName != "" {
		router.Use(NewRelicMiddleware(config.NewRelicName, config.NewRelicKey))
	}

	// Set up routes.
	router.GET("/status", GetStatus)
	router.GET("/stats", GetStats)
	router.GET("/count", GetCount)
	router.GET("/keys", GetKeys)
	router.GET("/doc", GetAll)
	router.PUT("/doc", PutBatch)
	router.GET("/doc/:key", GetDoc)
	router.PUT("/doc/:key", PutDoc)
	router.DELETE("/doc/:key", DeleteDoc)

	// Start the service. If this returns, it's an exceptional case.
	err = router.Run(config.BindAddr)
	log.Printf(err.Error())
	panic(err)

}
