package drive115

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	sdk "github.com/xhofe/115-sdk-go"
)

var ErrUploadUncertain = errors.New("115 upload result is uncertain")
var ossBucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

type UploadProgress func(done, total int64)

func uploadLocalFile(ctx context.Context, accessToken, refreshToken, path, destinationID string, progress UploadProgress, onRefresh func(string, string)) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("upload source is not a regular file")
	}
	full, pre, err := uploadHashes(file)
	if err != nil {
		return err
	}
	httpClient := &http.Client{Timeout: 30 * time.Second, CheckRedirect: rejectCrossAddressRedirect}
	client := sdk.New(sdk.WithAccessToken(accessToken), sdk.WithRefreshToken(refreshToken), sdk.WithOnRefreshToken(onRefresh))
	client.SetHttpClient(httpClient).SetUserAgent("Media-Hub/115-open")
	init, err := client.UploadInit(ctx, &sdk.UploadInitReq{FileName: info.Name(), FileSize: info.Size(), Target: destinationID, FileID: full, PreID: pre})
	if err != nil {
		return ErrUploadUncertain
	}
	if init.Status == 2 {
		if progress != nil {
			progress(info.Size(), info.Size())
		}
		return nil
	}
	if init.Status == 6 || init.Status == 7 || init.Status == 8 {
		init, err = verifyUploadInit(ctx, client, destinationID, info.Name(), info.Size(), full, pre, init, file)
		if err != nil {
			return ErrUploadUncertain
		}
		if init.Status == 2 {
			if progress != nil {
				progress(info.Size(), info.Size())
			}
			return nil
		}
	}
	if init.Bucket == "" || init.Object == "" || len(init.Callback.Value.Callback) == 0 {
		return ErrUploadUncertain
	}
	token, err := client.UploadGetToken(ctx)
	if err != nil {
		return ErrUploadUncertain
	}
	if err := validateOSSTarget(token.Endpoint, init.Bucket); err != nil {
		return err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err = ossUploadFile(ctx, file, info.Size(), token, init, progress); err != nil {
		return ErrUploadUncertain
	}
	return nil
}

func uploadHashes(r io.ReadSeeker) (string, string, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	full := sha1.New()
	if _, err := io.Copy(full, r); err != nil {
		return "", "", err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	pre := sha1.New()
	if _, err := io.Copy(pre, io.LimitReader(r, 128*1024)); err != nil {
		return "", "", err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	return strings.ToUpper(hex.EncodeToString(full.Sum(nil))), strings.ToUpper(hex.EncodeToString(pre.Sum(nil))), nil
}
func verifyUploadInit(ctx context.Context, client *sdk.Client, destination, name string, size int64, full, pre string, init *sdk.UploadInitResp, r io.ReadSeeker) (*sdk.UploadInitResp, error) {
	parts := strings.Split(init.SignCheck, "-")
	if len(parts) != 2 {
		return nil, errors.New("invalid upload sign range")
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, err
	}
	end, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || start < 0 || end < start || end >= size {
		return nil, errors.New("invalid upload sign range")
	}
	if _, err = r.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	hash := sha1.New()
	if _, err = io.CopyN(hash, r, end-start+1); err != nil {
		return nil, err
	}
	return client.UploadInit(ctx, &sdk.UploadInitReq{FileName: name, FileSize: size, Target: destination, FileID: full, PreID: pre, SignKey: init.SignKey, SignVal: strings.ToUpper(hex.EncodeToString(hash.Sum(nil)))})
}
func validateOSSTarget(endpoint, bucket string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("invalid 115 OSS endpoint")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || (!strings.HasSuffix(host, ".aliyuncs.com") && !strings.HasSuffix(host, ".aliyuncs.com.cn")) {
		return errors.New("invalid 115 OSS endpoint")
	}
	if !ossBucketPattern.MatchString(bucket) {
		return errors.New("invalid 115 OSS bucket")
	}
	return nil
}
func rejectCrossAddressRedirect(request *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	previous := via[len(via)-1].URL
	if !strings.EqualFold(previous.Scheme, request.URL.Scheme) || !strings.EqualFold(previous.Host, request.URL.Host) {
		return http.ErrUseLastResponse
	}
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	return nil
}
func ossUploadFile(ctx context.Context, r io.ReadSeeker, size int64, token *sdk.UploadGetTokenResp, init *sdk.UploadInitResp, progress UploadProgress) error {
	client, err := oss.New(token.Endpoint, token.AccessKeyId, token.AccessKeySecret, oss.SecurityToken(token.SecurityToken), oss.Timeout(15, 120))
	if err != nil {
		return err
	}
	bucket, err := client.Bucket(init.Bucket)
	if err != nil {
		return err
	}
	callback := oss.Callback(base64.StdEncoding.EncodeToString([]byte(init.Callback.Value.Callback)))
	callbackVar := oss.CallbackVar(base64.StdEncoding.EncodeToString([]byte(init.Callback.Value.CallbackVar)))
	if size <= 20*1024*1024 {
		return bucket.PutObject(init.Object, &uploadProgressReader{ctx: ctx, r: r, total: size, progress: progress}, callback, callbackVar)
	}
	imur, err := bucket.InitiateMultipartUpload(init.Object, oss.Sequential())
	if err != nil {
		return err
	}
	partSize := uploadPartSize(size)
	count := int((size + partSize - 1) / partSize)
	parts := make([]oss.UploadPart, 0, count)
	var done int64
	for number := 1; number <= count; number++ {
		if err := ctx.Err(); err != nil {
			_ = bucket.AbortMultipartUpload(imur)
			return err
		}
		offset := int64(number-1) * partSize
		current := partSize
		if offset+current > size {
			current = size - offset
		}
		if _, err = r.Seek(offset, io.SeekStart); err != nil {
			_ = bucket.AbortMultipartUpload(imur)
			return err
		}
		reader := &uploadProgressReader{ctx: ctx, r: io.LimitReader(r, current), done: done, total: size, progress: progress}
		part, err := bucket.UploadPart(imur, reader, current, number)
		if err != nil {
			_ = bucket.AbortMultipartUpload(imur)
			return err
		}
		done += current
		parts = append(parts, part)
	}
	_, err = bucket.CompleteMultipartUpload(imur, parts, callback, callbackVar)
	return err
}
func uploadPartSize(size int64) int64 {
	const mb = int64(1024 * 1024)
	part := int64(20) * mb
	for (size+part-1)/part > 9000 {
		part *= 2
	}
	return part
}

type uploadProgressReader struct {
	ctx         context.Context
	r           io.Reader
	done, total int64
	progress    UploadProgress
}

func (r *uploadProgressReader) Read(value []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(value)
	r.done += int64(n)
	if n > 0 && r.progress != nil {
		r.progress(r.done, r.total)
	}
	return n, err
}
