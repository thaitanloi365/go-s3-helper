package s3

import (
	"compress/gzip"
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"path/filepath"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/rs/xid"
)

// UploadFilesParams params
type UploadFilesParams struct {
	Bucket      string
	Folder      string
	UploadFiles []*multipart.FileHeader
}

// UploadAndCompressGzipFiles upload
func (wrapper *Wrapper) UploadAndCompressGzipFiles(params UploadFilesParams) []string {
	var uploadedFiles []string

	var cfg = wrapper.Config

	if params.Bucket == "" {
		panic("Log bucket is invalid")
	}

	for _, file := range params.UploadFiles {
		fmt.Printf("uploading file: %s\n", file.Filename)

		originFile, err := file.Open()
		if err != nil {
			fmt.Printf("failed to open file %q, %v", file, err)
			break
		}

		reader, writer := io.Pipe()
		go func() {
			gw := gzip.NewWriter(writer)
			io.Copy(gw, originFile)
			originFile.Close()
			gw.Close()
			writer.Close()
		}()
		var ext = path.Ext(file.Filename)
		var fileName = file.Filename[0 : len(file.Filename)-len(ext)]
		var gzipFileName = fmt.Sprintf("%s.gz", fileName)

		if err != nil {
			fmt.Printf("create s3 session error: %v\n", err)
			break
		}

		var fileKey = filepath.Base(gzipFileName)
		uploader := s3manager.NewUploader(wrapper.session)
		result, err := uploader.Upload(&s3manager.UploadInput{
			Body:   reader,
			Bucket: aws.String(params.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/%s", params.Folder, fileKey)),
			ACL:    aws.String(cfg.Credentials.ACL),
		})

		if err == nil {
			fmt.Printf("%s is uploaded to s3 at %s\n", fileKey, result.Location)
			uploadedFiles = append(uploadedFiles, gzipFileName)
			uploadedFiles = append(uploadedFiles, file.Filename)
		} else {
			fmt.Printf("update s3 error: %v\n", err)
		}

	}
	return uploadedFiles
}

// UploadFiles upload files
func (wrapper *Wrapper) UploadFiles(params UploadFilesParams) (uploadedFiles []string, errs []error) {

	if params.Bucket == "" {
		panic("Log bucket is invalid")
	}

	var wg sync.WaitGroup
	for _, f := range params.UploadFiles {
		wg.Add(1)
		go func(file *multipart.FileHeader, wg *sync.WaitGroup) {
			defer wg.Done()

			uploadedFile, err := wrapper.UploadFile(file, params.Folder, "")
			uploadedFiles = append(uploadedFiles, uploadedFile)
			errs = append(errs, err)
		}(f, &wg)
	}
	wg.Wait()
	return
}

// UploadFile upload file
func (wrapper *Wrapper) UploadFile(file *multipart.FileHeader, folder, key string) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	fmt.Printf("uploading file: %s\n", file.Filename)
	var k = key
	if k == "" {
		k = xid.New().String()
	}

	uploader := s3manager.NewUploader(wrapper.session)
	result, err := uploader.Upload(&s3manager.UploadInput{
		Body:   src,
		Bucket: aws.String(wrapper.Config.Credentials.Bucket),
		Key:    aws.String(fmt.Sprintf("%s/%s", folder, k)),
		ACL:    aws.String(wrapper.Config.Credentials.ACL),
	})
	if err != nil {
		return "", err
	}

	return result.Location, nil

}
