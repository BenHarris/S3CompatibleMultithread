package main

// cPanel backup transport helper for S3 Compatible Object Storage
// By Jack Hayhurst

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"time"
	//"log"
	"net/http"
	"os"

	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
	// "gopkg.in/amz.v1/aws"
	// "gopkg.in/amz.v1/s3"
)

const contentType = "application/octet-stream"

const pagesize = 1000

const chunkSize = 33554432 // 32M in bytes

var junkTimestamp time.Time

func init() {
	var err error
	junkTimestamp, err = time.Parse(time.RFC3339, "2011-01-01T08:00:00Z")
	if err != nil {
		log.Fatal(err)
	}
}

type runningConfig struct {
	Command    string
	Pwd        string
	bucketName string
	CmdParams  []string
	AccessKey  string
	SecretKey  string
	bucket     s3.Bucket
	output     io.Writer
}

func getConfig() (runningConfig, error) {
	if len(os.Args) < 6 || len(os.Args) > 7 {
		return runningConfig{}, fmt.Errorf("incorrect number of arguments - [%d]", len(os.Args))
	}
	config := new(runningConfig)
	// parameters are passed as:
	// binary Command Pwd [CmdParams ...] Bucket AccessKey
	config.Command = os.Args[1]

	switch len(os.Args) {
	case 6:
		switch config.Command {
		case "delete":
		case "chdir":
		case "mkdir":
		case "ls":
		case "rmdir":
		default:
			return runningConfig{}, fmt.Errorf("incorrect number of arguments for a %s - [%d]", config.Command, len(os.Args))
		}
	case 7:
		switch config.Command {
		case "get":
		case "put":
		default:
			return runningConfig{}, fmt.Errorf("incorrect number of arguments for a %s - [%d]", config.Command, len(os.Args))
		}
	default:
		return runningConfig{}, fmt.Errorf("invalid command %s", config.Command)
	}
	//Pwd := os.Args[2]
	config.bucketName = os.Args[len(os.Args)-2]
	config.AccessKey = os.Args[len(os.Args)-1]

	config.CmdParams = os.Args[3 : len(os.Args)-2]

	// SecretKey is passed via enviroment variable
	config.SecretKey = os.Getenv("PASSWORD")

	config.output = os.Stdout

	return *config, nil
}

func (c *runningConfig) SetupBucket() error {
	connection, err := c.SetupConnection()
	if err != nil {
		return err
	}

	c.bucket = *connection.Bucket(c.bucketName)
	return nil
}

func (c *runningConfig) SetupConnection() (*s3.S3, error) {
	bucketRegion := aws.Region{
		Name:              "backblaze",
		S3Endpoint:        "https://s3.eu-central-003.backblazeb2.com",
		S3LowercaseBucket: true,
	}

	bucketAuth, err := aws.GetAuth(c.AccessKey, c.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("Problem creating Authentication %s - %v", c.AccessKey, err)
	}

	return s3.New(bucketAuth, bucketRegion), nil
}

func ValidBucket(bucketName string, connection *s3.S3) (bool, error) {
	allBuckets, err := connection.ListBuckets()
	if err != nil {
		return false, fmt.Errorf("problem listing buckets - %v", err)
	}

	var bucketExists bool
	for _, bucket := range allBuckets.Buckets {
		if bucket.Name == bucketName {
			bucketExists = true
		}
	}
	return bucketExists, nil
}

func (c *runningConfig) callFunc() error {
	// call the function with the name of the Command that you got

	switch c.Command {
	case "chdir":
		return c.Chdir(c.CmdParams[0])
	case "ls":
		return c.Lsdir(c.CmdParams[0])
	case "get":
		return c.magicGet(c.CmdParams[1], c.CmdParams[0])
	case "put":
		return c.magicPut(c.CmdParams[1], c.CmdParams[0])
	case "mkdir":
		return c.Mkdir(c.CmdParams[0])
	case "rmdir":
		return c.Rmdir(c.CmdParams[0])
	case "delete":
		return c.delete(c.CmdParams[0])
	}
	return nil
}

func (c *runningConfig) cleanRemotePath(path string) string {
	return strings.TrimPrefix(filepath.Join(c.Pwd, path), string(os.PathSeparator))
}

// Mkdir creates a given folder on the remote server
// cli: `binary` `mkdir` `Pwd` `path` `bucketName` `username`
func (c *runningConfig) Mkdir(dir string) error {
	// err := c.bucket.Put(dir, []byte{}, contentType, "0700")
	// if err != nil {
	// 	return fmt.Errorf("failed to create the given folder %s - %v", dir, err)
	// }
	return nil
}

// Chdir does almost nothing - not required, but must return the path.
// cli: `binary` `chdir` `Pwd` `path` `bucketName` `username`
func (c *runningConfig) Chdir(dir string) error {
	_, err := fmt.Fprintln(c.output, dir)
	if err != nil {
		return fmt.Errorf("failed to print the given path %s - %v", dir, err)
	}
	return nil
}

// lists the content of a directory on the remote system
// cli: `binary` `ls` `Pwd` `path` `bucketName` `username`
// passed to this is ["path"]
func (c *runningConfig) Lsdir(dir string) error {
	dir = c.cleanRemotePath(dir) + string(os.PathSeparator)

	items, err := c.bucket.List(dir, "/", "", pagesize)
	if err != nil {
		return fmt.Errorf("failed to list the contents of path %s - %v", dir, err)
	}

	outputFormat := "Jan 02 15:04"

	// cPanel requires the total line from the top of some ls -l output
	fmt.Fprintf(c.output, "total %d\n",
		len(items.CommonPrefixes)+len(items.Contents))

	for _, target := range items.CommonPrefixes {
		_, err = fmt.Fprintf(c.output, "drwxr-xr-x 63 %s %s %d %s %s\n",
			"dir",
			"dir",
			63,
			junkTimestamp.Format(outputFormat),
			strings.TrimPrefix(target, dir),
		)
		if err != nil {
			return fmt.Errorf("failed display the folder %s - %v", target, err)
		}
	}

	for _, target := range items.Contents {
		cleanLibTs := strings.TrimSuffix(target.LastModified, ".000Z")
		cleanLibTs += "Z"
		fileTime, err := time.Parse(time.RFC3339, cleanLibTs)
		if err != nil {
			return err
		}

		// prints out in the format defined by:
		// "-rwxr-xr-1 root root 3171 Jan 18 12:23 temp.txt"
		_, err = fmt.Fprintf(c.output, "-rwxr-xr-x 1 %s %s %d %s %s\n",
			target.Owner.ID,
			target.Owner.ID,
			target.Size,
			fileTime.Format(outputFormat),
			strings.TrimPrefix(target.Key, dir),
		)
		if err != nil {
			return fmt.Errorf("failed display the file %s - %v", target.Key, err)
		}
	}
	return nil
}

// Gets a larger file from the remote location and puts it on the local system
// cli: `binary` `get` `Pwd `Remote file` `local file` `bucketName` `username`
// passed to this is ["remote file", "local file"]
func (c *runningConfig) magicGet(local, remote string) error {
	// open up the output file for writing
	outFile, err := os.OpenFile(local, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	defer outFile.Close()
	if err != nil {
		return fmt.Errorf("error writing to local file %s - %v", local, err)
	}

	// open up the remote file for reading
	dataResponse, err := c.bucket.GetResponse(remote)
	defer dataResponse.Body.Close()
	if err != nil {
		return fmt.Errorf("error loading remote file %s - %v", remote, err)
	}

	// copy all bytes, without loading stuff in memory, then defer close happen
	_, err = io.Copy(outFile, dataResponse.Body)
	if err != nil {
		return fmt.Errorf("error writing to local file %s - %v", local, err)
	}

	return nil
}

// puts a larger file from the local location to a remote location by pieces
// cli: `binary` `put` `Pwd `local file` `remote file` `bucketName` `username`
func (c *runningConfig) magicPut(remote, local string) error {
	// open the file to be transferred
	file, err := os.Open(local)
	defer file.Close()
	if err != nil {
		return fmt.Errorf("error loading local file %s - %v", local, err)
	}

	bytes := make([]byte, chunkSize)
	buffer := bufio.NewReader(file)
	// at most, buffer.Read can only read len(bytes) bytes
	_, err = buffer.Read(bytes)
	if err != nil {
		return fmt.Errorf("error reading from local file %s - %v", local, err)
	}

	// determine the filetype based on the bytes you read
	filetype := http.DetectContentType(bytes)
	_, err = file.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("error resetting local file %s - %v", local, err)
	}

	// set up for multipart upload
	multiUploader, err := c.bucket.InitMulti(remote, filetype, s3.ACL("private"))
	if err != nil {
		return fmt.Errorf("error opening remote file %s - %v", remote, err)
	}

	// upload all of the file in pieces
	parts, err := multiUploader.PutAll(file, chunkSize)
	if err != nil {
		return fmt.Errorf("error writing to remote file %s - %v", local, err)
	}

	// complete the file
	err = multiUploader.Complete(parts)
	if err != nil {
		return fmt.Errorf("error completing file %s - %v", remote, err)
	}

	return nil
}

// removes everything under the given path on the remote Bucket
// cli: `binary` `Rmdir` `Pwd` `path` `bucketName` `username`
// passed to this is ["path"]
func (c *runningConfig) Rmdir(target string) error {
	if !strings.HasSuffix(target, string(os.PathSeparator)) {
		target += string(os.PathSeparator)
	}

	items, err := c.bucket.List(target, "", "", pagesize)
	if err != nil {
		return fmt.Errorf("error listing path %s - %v", target, err)
	}
	for len(items.Contents) > 0 {
		for _, target := range items.Contents {
			err = c.bucket.Del(target.Key)
			if err != nil {
				return fmt.Errorf("error removing remote %s - %v", target.Key, err)
			}
		}
		// check to make sure everything is gone
		items, err = c.bucket.List(target, "", "", pagesize)
		if err != nil {
			return fmt.Errorf("error listing path %s - %v", target, err)
		}
	}
	return nil
}

// removes a file at a given location
// cli: `delete` `rmdir` `Pwd` `path` `bucketName` `username`
// passed to this is ["path"]
func (c *runningConfig) delete(remote string) error {
	err := c.bucket.Del(remote)
	if err != nil {
		return fmt.Errorf("failed to delete %s - %v", remote, err)
	}
	return nil
}

func main() {

	config, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}

	//connection := SetupConnection(config)
	err = config.SetupBucket()
	if err != nil {
		log.Fatal(err)
	}

	if err = config.callFunc(); err != nil {
		log.Fatal(err)
	}
}
