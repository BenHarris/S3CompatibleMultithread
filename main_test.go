package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sebdah/goldie"
)

func init() {
	goldie.FixtureDir = "testdata/fixtures"
}

func loadTestingConfig(t *testing.T) (*runningConfig, *bytes.Buffer) {
	testingConfig := new(runningConfig)
	testingConfig.Pwd = os.Getenv("PWD")
	testingConfig.AccessKey = os.Getenv("ACCESSKEY")
	testingConfig.SecretKey = os.Getenv("SECRETKEY")
	testingConfig.bucketName = os.Getenv("BUCKET")
	if testingConfig.Pwd == "" ||
		testingConfig.AccessKey == "" ||
		testingConfig.SecretKey == "" ||
		testingConfig.bucketName == "" {
		t.Skip("missing test configuration")
	}

	// set up something to capture output
	outputBuf := bytes.Buffer{}
	testingConfig.output = &outputBuf

	return testingConfig, &outputBuf
}

func TestGetConfig(t *testing.T) {
	os.Args = []string{"junk", "ls", "/pwd", "subpath/is/here", "pail", "bob"}
	err := os.Setenv("PASSWORD", "sekret")
	assert.NoError(t, err)

	config, err := getConfig()
	assert.NoError(t, err)

	expectedConfig := runningConfig{
		Command:    "ls",
		Pwd:        "",
		bucketName: "pail",
		AccessKey:  "bob",
		SecretKey:  "sekret",
		output:     os.Stdout,
		CmdParams: []string{
			"subpath/is/here",
		},
	}

	assert.Equal(t, expectedConfig, config)
}

func TestSetupConnection(t *testing.T) {
	testingConfig := &runningConfig{
		Pwd:        "/",
		AccessKey:  "AccEssKey",
		SecretKey:  "SecRetKey",
		bucketName: "BuKKiT",
	}
	connection, err := testingConfig.SetupConnection()
	assert.NoError(t, err)

	assert.Equal(t, "AccEssKey", connection.Auth.AccessKey, "the Access Key should be the same")
	assert.Equal(t, "SecRetKey", connection.Auth.SecretKey, "the Secret Key should be the same")
	assert.Equal(t, "https://s3.eu-central-003.backblazeb2.com", connection.Region.S3Endpoint, "the URL should be LW's")
	assert.Equal(t, "backblaze", connection.Region.Name, "the URL should be LW's")
}

func TestRunningConfig_SetupBucket(t *testing.T) {
	testingConfig := &runningConfig{
		Pwd:        "/",
		AccessKey:  "AccEssKey",
		SecretKey:  "SecRetKey",
		bucketName: "BuKKiT",
	}
	err := testingConfig.SetupBucket()
	assert.NoError(t, err)

	assert.Equal(t, "AccEssKey", testingConfig.bucket.S3.Auth.AccessKey, "the Access Key should be the same")
	assert.Equal(t, "SecRetKey", testingConfig.bucket.S3.Auth.SecretKey, "the Secret Key should be the same")
	assert.Equal(t, "https://s3.eu-central-003.backblazeb2.com", testingConfig.bucket.S3.Region.S3Endpoint, "the URL should be LW's")
	assert.Equal(t, "backblaze", testingConfig.bucket.S3.Region.Name, "the URL should be LW's")
	assert.Equal(t, "bukkit", testingConfig.bucket.Name, "the name of the bucket is not being set correctly")
}

func TestHiddenConfig(t *testing.T) {
	testingConfig, _ := loadTestingConfig(t)
	//connection := SetupConnection(testingConfig)
	err := testingConfig.SetupBucket()
	assert.NoError(t, err)

	assert.Equal(t, testingConfig.AccessKey, testingConfig.bucket.S3.Auth.AccessKey, "the Access Key should be the same")
	assert.Equal(t, testingConfig.SecretKey, testingConfig.bucket.S3.Auth.SecretKey, "the Secret Key should be the same")
	assert.Equal(t, testingConfig.bucketName, testingConfig.bucket.Name, "the name of the bucket is not being set correctly")
	assert.Equal(t, "https://s3.eu-central-003.backblazeb2.com", testingConfig.bucket.S3.Region.S3Endpoint, "the URL should be LW's")
	assert.Equal(t, "backblaze", testingConfig.bucket.S3.Region.Name, "the URL should be LW's")
}

func TestRunningConfig_ValidBucket(t *testing.T) {
	testingConfig, _ := loadTestingConfig(t)
	connection, err := testingConfig.SetupConnection()
	assert.NoError(t, err)

	_, err = connection.ListBuckets()
	assert.NoError(t, err)

	bucketExists, err := ValidBucket(testingConfig.bucketName, connection)
	assert.True(t, bucketExists, "the bucket should exist within the given space")
	assert.NoError(t, err)

	bucketExists, err = ValidBucket("BadBucket", connection)
	assert.False(t, bucketExists, "the BadBucket should not exist within the given space")
	assert.NoError(t, err)
}

func TestRunningConfig_Chdir(t *testing.T) {
	outputBuf := bytes.Buffer{}
	testingConfig := runningConfig{
		Pwd:        "/",
		AccessKey:  "AccEssKey",
		SecretKey:  "SecRetKey",
		bucketName: "BuKKiT",
		output:     &outputBuf,
	}

	testingConfig.SetupBucket()

	testingConfig.CmdParams = []string{"/"}
	testingConfig.Chdir(testingConfig.CmdParams[0])

	testingConfig.CmdParams = []string{"/folderthatdoesnotexist"}
	testingConfig.Chdir(testingConfig.CmdParams[0])

	testingConfig.CmdParams = []string{"/testing"}
	testingConfig.Chdir(testingConfig.CmdParams[0])

	goldie.Assert(t, t.Name(), outputBuf.Bytes())
}

func testList(t *testing.T, c *runningConfig) {
	items, err := c.bucket.List("testdata/", "/", "", pagesize)
	assert.NoError(t, err)

	for _, target := range items.Contents {
		fmt.Fprintf(c.output, "%d [%s]\n", target.Size, target.Key)
	}
}

func TestRemoteFolder(t *testing.T) {
	files := []string{
		"thank_you_for_not_loitering.jpg",
		"database.tar.gz",
	}

	for i := 0; i < len(files); i++ {
		files[i] = path.Clean(files[i])
	}

	testingConfig, outputBuf := loadTestingConfig(t)
	err := testingConfig.SetupBucket()
	assert.NoError(t, err)

	err = testingConfig.Rmdir("testdata")
	assert.NoError(t, err)

	err = testingConfig.Mkdir("testdata")
	assert.NoError(t, err)

	for _, file := range files {
		local := filepath.Join("testdata", file)
		remote := filepath.Join("testdata", file)
		fmt.Fprintf(testingConfig.output, "\nputting local [%s] into remote [%s]\n", local, remote)

		err = testingConfig.magicPut(remote, local)
		assert.NoError(t, err)

		testList(t, testingConfig)
	}

	tmpdir, err := ioutil.TempDir("", "cPanel_backup_transporter")
	assert.NoError(t, err)

	for _, file := range files {
		local := filepath.Join(tmpdir, file)
		remote := filepath.Join("testdata", file)

		err = testingConfig.magicGet(local, remote)
		assert.NoError(t, err)
	}

	for _, file := range files {
		fi, err := os.Stat(filepath.Join(tmpdir, file))
		assert.NoError(t, err)

		fmt.Fprintf(testingConfig.output, "[%d] %s\n", fi.Size(), fi.Name())
	}
	fmt.Fprintln(testingConfig.output, "contents before removal")

	testList(t, testingConfig)

	fmt.Fprintln(testingConfig.output, "removing the first file")

	err = testingConfig.delete(filepath.Join("testdata", files[0]))
	assert.NoError(t, err)

	testList(t, testingConfig)

	err = testingConfig.Rmdir("testdata")
	assert.NoError(t, err)

	fmt.Fprintln(testingConfig.output, "contents after removal")

	testList(t, testingConfig)

	goldie.Assert(t, t.Name(), outputBuf.Bytes())
}

func TestRunningConfig_Lsdir(t *testing.T) {
	r, w := io.Pipe()
	c, _ := loadTestingConfig(t)
	err := c.SetupBucket()
	assert.NoError(t, err)

	prefix := "-rwxr-xr-x"

	c.output = w

	var pastFirstLine bool

	scanner := bufio.NewScanner(r)
	go func() {
		for scanner.Scan() {
			if !pastFirstLine {
				assert.True(t, strings.HasPrefix(scanner.Text(), "total"), "missing first total line")
				pastFirstLine = true
				continue
			}
			assert.True(t, strings.HasPrefix(scanner.Text(), prefix), "missing prefix - [%s]\n[%s]", prefix, scanner.Text)
			fmt.Fprintln(os.Stderr, scanner.Text()) // print out each line for manual inspection
		}
		assert.NoError(t, scanner.Err())
	}()

	err = c.Rmdir("testdata")
	assert.NoError(t, err)

	err = c.Mkdir("testdata")
	assert.NoError(t, err)
	files := []string{
		"thank_you_for_not_loitering.jpg",
		"database.tar.gz",
	}

	for _, file := range files {
		local := filepath.Join("testdata", file)
		remote := filepath.Join("testdata", file)

		err = c.magicPut(remote, local)
		assert.NoError(t, err)
	}

	err = c.Lsdir("testdata")
	assert.NoError(t, err)

	w.Close()

	err = c.Rmdir("testdata")
	assert.NoError(t, err)
}
