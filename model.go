package s3

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

const policyDocument = `
{ "expiration": "%s",
  "conditions": [
    {"bucket": "%s"},
    ["starts-with", "$key", "%s"],
    {"acl": "public-read"},
    ["content-length-range", 1, %d],
    {"x-amz-credential": "%s"},
    {"x-amz-algorithm": "AWS4-HMAC-SHA256"},
    {"x-amz-date": "%s" }
  ]
}
`
const (
	expirationFormat = "2006-01-02T15:04:05.000Z"
	timeFormat       = "20060102T150405Z"
	shortTimeFormat  = "20060102"
	acl              = "public-read"
	algorithm        = "AWS4-HMAC-SHA256"
)

// Credentials Represents AWS credentials and config.
type Credentials struct {
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	ACL       string
	LogBucket string
}

// PolicyOptions Represents policy options.
type PolicyOptions struct {
	ExpiryMinutes int
	MaxFileSize   int
}

// Config config
type Config struct {
	Credentials   Credentials
	PolicyOptions PolicyOptions
}

// Signature response
type Signature struct {
	Key         string `json:"key"`
	URL         string `json:"url"`
	Policy      string `json:"policy"`
	Credential  string `json:"x-amz-credential"`
	Algorithm   string `json:"x-amz-algorithm"`
	Signature   string `json:"x-amz-signature"`
	Date        string `json:"x-amz-date"`
	ACL         string `json:"acl"`
	ContentType string `json:"content-type"`
}

// Wrapper wrapper
type Wrapper struct {
	Config  *Config
	session *session.Session
}

// Extension ext
type Extension string

// Defined
var (
	JPGExtension  Extension = ".jpg"
	JPEGExtension Extension = ".jpeg"
	PNGExtension  Extension = ".png"
	GIFExtension  Extension = ".gif"
)

// ImageContentType content-type
type ImageContentType string

// Defined
var (
	PNGImageContentType  ImageContentType = "image/png"
	JPEGImageContentType ImageContentType = "image/jpeg"
	JPGImageContentType  ImageContentType = "image/jpg"
	GIFImageContentType  ImageContentType = "image/gif"
)

// ImageBase64 base64
type ImageBase64 string

// GetRawBase64 raw base64
func (imageBase64 ImageBase64) GetRawBase64() string {
	var data = string(imageBase64)
	var b64data = data[strings.IndexByte(data, ',')+1:]
	return b64data
}

// GetContentType get content type
func (imageBase64 ImageBase64) GetContentType() ImageContentType {
	var data = string(imageBase64)
	var from = strings.Index(string(data), ",")
	var suffix = strings.TrimSuffix(data[5:from], ";base64")
	switch suffix {

	case "image/png":
		return PNGImageContentType
	case "image/jpg":
		return JPGImageContentType
	case "image/jpeg":
		return JPEGImageContentType
	case "image/gif":
		return GIFImageContentType
	}
	return PNGImageContentType
}

// GetExtionsion ext
func (imageBase64 ImageBase64) GetExtionsion() Extension {
	switch imageBase64.GetContentType() {
	case JPEGImageContentType:
		return JPEGExtension
	case JPGImageContentType:
		return JPGExtension
	case PNGImageContentType:
		return PNGExtension
	case GIFImageContentType:
		return GIFExtension
	default:
		break
	}

	return PNGExtension
}

// New instance
func New(config *Config) *Wrapper {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(config.Credentials.Region),
		Credentials: credentials.NewStaticCredentials(config.Credentials.AccessKey, config.Credentials.SecretKey, ""),
	})

	if err != nil {
		panic("Create aws session error")
		fmt.Println("err", err)
	}
	return &Wrapper{
		Config:  config,
		session: sess,
	}

}
