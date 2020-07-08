package s3

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/disintegration/imaging"
	"github.com/rs/xid"
)

// ResizeImageParams resize params
type ResizeImageParams struct {
	MaxWidthToResize int
	Width            int
	Height           int
	Bucket           string
	Folder           string
}

// UploadFilesParams params
type UploadFilesParams struct {
	Bucket      string
	Dir         string
	Folder      string
	IgnoreFiles []string
}

// UploadImage64Params params
type UploadImage64Params struct {
	ImageBase64       string
	Bucket            string
	Folder            string
	ResizeImageParams *ResizeImageParams
}

// UploadImageResult result
type UploadImageResult struct {
	URL    string
	Width  int
	Height int
}

// UploadImage64Result result
type UploadImage64Result struct {
	UploadImageResult
	ResizeImage *UploadImageResult
}

// UploadFiles upload
func (wrapper *Wrapper) UploadFiles(params UploadFilesParams) []string {
	var uploadedFiles []string
	var files []string

	var cfg = wrapper.Config

	if params.Bucket == "" {
		panic("Log bucket is invalid")
	}

	fmt.Println("ignoreFiles", params.IgnoreFiles)
	var err = filepath.Walk(params.Dir, func(path string, info os.FileInfo, err error) error {
		for _, ignoreFile := range params.IgnoreFiles {
			if info.IsDir() == false && strings.Index(path, ignoreFile) == -1 {
				files = append(files, path)
			}
		}
		return nil
	})

	fmt.Println("files to update", files)
	if err == nil {
		for _, file := range files {
			fmt.Printf("uploading file: %s\n", file)

			originFile, err := os.Open(file)
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
			var ext = path.Ext(file)
			var fileName = file[0 : len(file)-len(ext)]
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
				uploadedFiles = append(uploadedFiles, file)
			} else {
				fmt.Printf("update s3 error: %v\n", err)
			}

		}
	}
	return uploadedFiles
}

// UploadFiles upload
func (wrapper *Wrapper) UploadFile(file *os.File, folder string) (string, error) {
	originFile, err := os.Open(file)
	if err != nil {
		fmt.Printf("failed to open file %q, %v", file, err)
		return "", err
	}

	var cfg = wrapper.Config

	fmt.Println("files to update", files)
	if err == nil {
		for _, file := range files {
			fmt.Printf("uploading file: %s\n", file)

			reader, writer := io.Pipe()
			go func() {
				gw := gzip.NewWriter(writer)
				io.Copy(gw, originFile)
				originFile.Close()
				gw.Close()
				writer.Close()
			}()
			var ext = path.Ext(file)
			var fileName = file[0 : len(file)-len(ext)]
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
				uploadedFiles = append(uploadedFiles, file)
			} else {
				fmt.Printf("update s3 error: %v\n", err)
			}

		}
	}
	return uploadedFiles
}

// UploadImageBase64 base64
func (wrapper *Wrapper) UploadImageBase64(params UploadImage64Params) (*UploadImage64Result, error) {
	var imgBase64 = ImageBase64(params.ImageBase64)
	rawData, err := base64.StdEncoding.DecodeString(imgBase64.GetRawBase64())

	if params.Bucket == "" {
		return nil, fmt.Errorf("Bucket is invalid")
	}
	var buffer = bytes.NewReader(rawData)
	var imageOrigin image.Image
	var contentType = imgBase64.GetContentType()
	switch contentType {
	case PNGImageContentType:
		imageOrigin, err = png.Decode(buffer)
		if err != nil {
			return nil, err
		}
	case JPGImageContentType, JPEGImageContentType:
		imageOrigin, err = jpeg.Decode(buffer)
		if err != nil {
			return nil, err
		}
	}

	var ext = imgBase64.GetExtionsion()
	var id = xid.New().String()
	var key = fmt.Sprintf("%s/%s%s", params.Folder, id, ext)

	var uploader = s3manager.NewUploader(wrapper.session)

	var size = imageOrigin.Bounds().Max
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(params.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewBuffer(rawData),
		ACL:         aws.String("public-read"),
		ContentType: aws.String(string(contentType)),
		Metadata: map[string]*string{
			"width":  aws.String(fmt.Sprintf("%d", size.X)),
			"height": aws.String(fmt.Sprintf("%d", size.Y)),
		},
	})
	if err != nil {
		return nil, err
	}

	var res = &UploadImage64Result{}
	res.URL = result.Location
	res.Width = size.X
	res.Height = size.Y

	if params.ResizeImageParams != nil {
		if res.Width > params.ResizeImageParams.MaxWidthToResize {
			var thumbnailWidth = params.ResizeImageParams.Width
			if params.ResizeImageParams.Width <= 0 {
				thumbnailWidth = 128
			}

			var thumbnailHeight = thumbnailWidth * size.Y / size.X
			if params.ResizeImageParams.Height > 0 {
				thumbnailHeight = params.ResizeImageParams.Height
			}
			var thumbnail = imaging.Resize(imageOrigin, thumbnailWidth, thumbnailHeight, imaging.Lanczos)
			var format imaging.Format
			switch contentType {
			case JPEGImageContentType, JPGImageContentType:
				format = imaging.JPEG
			case PNGImageContentType:
				format = imaging.PNG
			}

			var bufferEncode = new(bytes.Buffer)
			err = imaging.Encode(bufferEncode, thumbnail, format)

			var key = fmt.Sprintf("%s/%s%s", params.ResizeImageParams.Folder, id, ext)
			if err == nil {
				result, err := uploader.Upload(&s3manager.UploadInput{
					Bucket:      aws.String(params.ResizeImageParams.Bucket),
					Key:         aws.String(key),
					Body:        bufferEncode,
					ACL:         aws.String("public-read"),
					ContentType: aws.String(string(contentType)),
					Metadata: map[string]*string{
						"width":  aws.String(fmt.Sprintf("%d", thumbnailWidth)),
						"height": aws.String(fmt.Sprintf("%d", thumbnailHeight)),
					},
				})
				if err == nil {
					res.ResizeImage = &UploadImageResult{
						URL:    result.Location,
						Width:  thumbnailWidth,
						Height: thumbnailHeight,
					}

				}
			}

		}
	}
	return res, nil
}
