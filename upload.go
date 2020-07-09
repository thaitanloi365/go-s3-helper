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
	"io/ioutil"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"

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
	Folder      string
	UploadFiles []*multipart.FileHeader
}

// UploadImage64Params params
type UploadImage64Params struct {
	ImageBase64       string
	Bucket            string
	Folder            string
	ResizeImageParams *ResizeImageParams
}

// UploadImageFileParams params
type UploadImageFileParams struct {
	ImageFile         *os.File
	Bucket            string
	Folder            string
	ResizeImageParams *ResizeImageParams
}

// uploadImageResult result
type uploadImageResult struct {
	URL    string
	Width  int
	Height int
}

// UploadImageResult result
type UploadImageResult struct {
	uploadImageResult
	ResizeImage *uploadImageResult
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
func (wrapper *Wrapper) UploadFiles(params UploadFilesParams) []string {
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

		data, err := ioutil.ReadAll(originFile)
		if err != nil {
			fmt.Printf("failed to read file %q, %v", file, err)
			break
		}

		if err != nil {
			fmt.Printf("create s3 session error: %v\n", err)
			break
		}

		var fileKey = filepath.Base(file.Filename)
		uploader := s3manager.NewUploader(wrapper.session)
		result, err := uploader.Upload(&s3manager.UploadInput{
			Body:   bytes.NewBuffer(data),
			Bucket: aws.String(params.Bucket),
			Key:    aws.String(fmt.Sprintf("%s/%s", params.Folder, fileKey)),
			ACL:    aws.String(cfg.Credentials.ACL),
		})

		if err == nil {
			fmt.Printf("%s is uploaded to s3 at %s\n", fileKey, result.Location)
			uploadedFiles = append(uploadedFiles, file.Filename)
		} else {
			fmt.Printf("update s3 error: %v\n", err)
		}

	}
	return uploadedFiles
}

// UploadImageFile FILE
func (wrapper *Wrapper) UploadImageFile(params UploadImageFileParams) (*UploadImageResult, error) {
	ct, err := getFileContentType(params.ImageFile)
	if err != nil {
		return nil, err
	}

	if params.Bucket == "" {
		return nil, fmt.Errorf("Bucket is invalid")
	}

	var ext = path.Ext(params.ImageFile.Name())
	var id = xid.New().String()
	var key = fmt.Sprintf("%s/%s%s", params.Folder, id, ext)

	var uploader = s3manager.NewUploader(wrapper.session)
	var imageOrigin image.Image
	var contentType = ImageContentType(ct)
	switch contentType {
	case PNGImageContentType:
		imageOrigin, err = png.Decode(params.ImageFile)
		if err != nil {
			return nil, err
		}
	case JPGImageContentType, JPEGImageContentType:
		imageOrigin, err = jpeg.Decode(params.ImageFile)
		if err != nil {
			return nil, err
		}
	}

	buffer, err := ioutil.ReadAll(params.ImageFile)
	if err != nil {
		return nil, err
	}

	var size = imageOrigin.Bounds().Max
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(params.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewBuffer(buffer),
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

	var res = &UploadImageResult{}
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
					res.ResizeImage = &uploadImageResult{
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

// UploadFile upload file
func (wrapper *Wrapper) UploadFile(fileName, folder string) (string, error) {

	file, err := os.Open(fileName)
	if err != nil {
		return "", err
	}

	uploader := s3manager.NewUploader(wrapper.session)
	result, err := uploader.Upload(&s3manager.UploadInput{
		Body:   file,
		Bucket: aws.String(wrapper.Config.Credentials.Bucket),
		Key:    aws.String(fmt.Sprintf("%s/%s", folder, xid.New().String())),
		ACL:    aws.String(wrapper.Config.Credentials.ACL),
	})
	if err != nil {
		return "", err
	}

	return result.Location, nil

}

// UploadImageBase64 base64
func (wrapper *Wrapper) UploadImageBase64(params UploadImage64Params) (*UploadImageResult, error) {
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

	var res = &UploadImageResult{}
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
					res.ResizeImage = &uploadImageResult{
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
