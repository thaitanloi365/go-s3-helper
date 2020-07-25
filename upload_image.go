package s3

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"mime/multipart"
	"path"
	"sync"

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

// UploadImage64Params params
type UploadImage64Params struct {
	Key               string
	ImageBase64       string
	Bucket            string
	Folder            string
	ResizeImageParams *ResizeImageParams
}

// UploadImageFileParams params
type UploadImageFileParams struct {
	Key               string
	ImageFile         *multipart.FileHeader
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

// UploadImageFile FILE
func (wrapper *Wrapper) UploadImageFile(params UploadImageFileParams) (*UploadImageResult, error) {
	f, err := params.ImageFile.Open()
	if err != nil {
		if err != nil {
			return nil, err
		}
	}

	ct, err := getFileContentType(f)
	if err != nil {
		return nil, err
	}

	if params.Bucket == "" {
		return nil, fmt.Errorf("Bucket is invalid")
	}

	var ext = path.Ext(params.ImageFile.Filename)
	var id = params.Key
	if id == "" {
		id = xid.New().String()
	}

	var key = fmt.Sprintf("%s/%s%s", params.Folder, id, ext)

	var uploader = s3manager.NewUploader(wrapper.session)
	var imageOrigin image.Image
	var contentType = ImageContentType(ct)
	switch contentType {
	case PNGImageContentType:
		imageOrigin, err = png.Decode(f)
		if err != nil {
			return nil, err
		}
	case JPGImageContentType, JPEGImageContentType:
		imageOrigin, err = jpeg.Decode(f)
		if err != nil {
			return nil, err
		}
	}

	buffer, err := ioutil.ReadAll(f)
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

// UploadImageFiles upload multi files
func (wrapper *Wrapper) UploadImageFiles(params []UploadImageFileParams) (results []*UploadImageResult, errs []error) {
	var wg sync.WaitGroup
	for _, p := range params {
		wg.Add(1)
		go func(p UploadImageFileParams) {
			r, err := wrapper.UploadImageFile(p)
			results = append(results, r)
			errs = append(errs, err)
		}(p)
	}

	return

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
