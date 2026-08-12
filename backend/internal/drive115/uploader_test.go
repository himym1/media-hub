package drive115

import "testing"

func TestValidateOSSTarget(t *testing.T) {
	for _, test := range []struct {
		endpoint, bucket string
		valid            bool
	}{{"https://oss-cn-shenzhen.aliyuncs.com", "bucket-1", true}, {"http://oss-cn-shenzhen.aliyuncs.com", "bucket", false}, {"https://example.com", "bucket", false}, {"https://oss-cn-shenzhen.aliyuncs.com?token=x", "bucket", false}, {"https://oss-cn-shenzhen.aliyuncs.com", "../bucket", false}, {"https://oss-cn-shenzhen.aliyuncs.com", "evil.example", false}} {
		err := validateOSSTarget(test.endpoint, test.bucket)
		if test.valid && err != nil {
			t.Fatalf("%s: %v", test.endpoint, err)
		}
		if !test.valid && err == nil {
			t.Fatalf("expected rejection for %s %s", test.endpoint, test.bucket)
		}
	}
}
