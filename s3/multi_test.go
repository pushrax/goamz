package s3_test

import (
	"encoding/xml"
	"github.com/pushrax/goamz/s3"
	"gopkg.in/check.v1"
	"io"
	"io/ioutil"
	"strings"
)

func (s *S) TestInitMulti(c *check.C) {
	testServer.Response(200, nil, InitMultiResultDump)

	b := s.s3.Bucket("sample")

	multi, err := b.InitMulti("multi", "text/plain", s3.Private)
	c.Assert(err, check.IsNil)

	req := testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "POST")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	c.Assert(req.Header["Content-Type"], check.DeepEquals, []string{"text/plain"})
	c.Assert(req.Header["X-Amz-Acl"], check.DeepEquals, []string{"private"})
	c.Assert(req.Form["uploads"], check.DeepEquals, []string{""})

	c.Assert(multi.UploadId, check.Matches, "JNbR_[A-Za-z0-9.]+QQ--")
}

func (s *S) TestMultiNoPreviousUpload(c *check.C) {
	// Don't retry the NoSuchUpload error.
	s.DisableRetries()

	testServer.Response(404, nil, NoSuchUploadErrorDump)
	testServer.Response(200, nil, InitMultiResultDump)

	b := s.s3.Bucket("sample")

	multi, err := b.Multi("multi", "text/plain", s3.Private)
	c.Assert(err, check.IsNil)

	req := testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "GET")
	c.Assert(req.URL.Path, check.Equals, "/sample/")
	c.Assert(req.Form["uploads"], check.DeepEquals, []string{""})
	c.Assert(req.Form["prefix"], check.DeepEquals, []string{"multi"})

	req = testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "POST")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	c.Assert(req.Form["uploads"], check.DeepEquals, []string{""})

	c.Assert(multi.UploadId, check.Matches, "JNbR_[A-Za-z0-9.]+QQ--")
}

func (s *S) TestMultiReturnOld(c *check.C) {
	testServer.Response(200, nil, ListMultiResultDump)

	b := s.s3.Bucket("sample")

	multi, err := b.Multi("multi1", "text/plain", s3.Private)
	c.Assert(err, check.IsNil)
	c.Assert(multi.Key, check.Equals, "multi1")
	c.Assert(multi.UploadId, check.Equals, "iUVug89pPvSswrikD")

	req := testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "GET")
	c.Assert(req.URL.Path, check.Equals, "/sample/")
	c.Assert(req.Form["uploads"], check.DeepEquals, []string{""})
	c.Assert(req.Form["prefix"], check.DeepEquals, []string{"multi1"})
}

func (s *S) TestListParts(c *check.C) {
	testServer.Response(200, nil, InitMultiResultDump)
	testServer.Response(200, nil, ListPartsResultDump1)
	testServer.Response(404, nil, NoSuchUploadErrorDump) // :-(
	testServer.Response(200, nil, ListPartsResultDump2)

	b := s.s3.Bucket("sample")

	multi, err := b.InitMulti("multi", "text/plain", s3.Private)
	c.Assert(err, check.IsNil)

	parts, err := multi.ListParts()
	c.Assert(err, check.IsNil)
	c.Assert(parts, check.HasLen, 3)
	c.Assert(parts[0].N, check.Equals, 1)
	c.Assert(parts[0].Size, check.Equals, int64(5))
	c.Assert(parts[0].ETag, check.Equals, `"ffc88b4ca90a355f8ddba6b2c3b2af5c"`)
	c.Assert(parts[1].N, check.Equals, 2)
	c.Assert(parts[1].Size, check.Equals, int64(5))
	c.Assert(parts[1].ETag, check.Equals, `"d067a0fa9dc61a6e7195ca99696b5a89"`)
	c.Assert(parts[2].N, check.Equals, 3)
	c.Assert(parts[2].Size, check.Equals, int64(5))
	c.Assert(parts[2].ETag, check.Equals, `"49dcd91231f801159e893fb5c6674985"`)
	testServer.WaitRequest()
	req := testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "GET")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	c.Assert(req.Form.Get("uploadId"), check.Matches, "JNbR_[A-Za-z0-9.]+QQ--")
	c.Assert(req.Form["max-parts"], check.DeepEquals, []string{"1000"})

	testServer.WaitRequest() // The internal error.
	req = testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "GET")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	c.Assert(req.Form.Get("uploadId"), check.Matches, "JNbR_[A-Za-z0-9.]+QQ--")
	c.Assert(req.Form["max-parts"], check.DeepEquals, []string{"1000"})
	c.Assert(req.Form["part-number-marker"], check.DeepEquals, []string{"2"})
}

func (s *S) TestPutPart(c *check.C) {
	headers := map[string]string{
		"ETag": `"26f90efd10d614f100252ff56d88dad8"`,
	}
	testServer.Response(200, nil, InitMultiResultDump)
	testServer.Response(200, headers, "")

	b := s.s3.Bucket("sample")

	multi, err := b.InitMulti("multi", "text/plain", s3.Private)
	c.Assert(err, check.IsNil)

	part, err := multi.PutPart(1, strings.NewReader("<part 1>"))
	c.Assert(err, check.IsNil)
	c.Assert(part.N, check.Equals, 1)
	c.Assert(part.Size, check.Equals, int64(8))
	c.Assert(part.ETag, check.Equals, headers["ETag"])

	testServer.WaitRequest()
	req := testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "PUT")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	c.Assert(req.Form.Get("uploadId"), check.Matches, "JNbR_[A-Za-z0-9.]+QQ--")
	c.Assert(req.Form["partNumber"], check.DeepEquals, []string{"1"})
	c.Assert(req.Header["Content-Length"], check.DeepEquals, []string{"8"})
	c.Assert(req.Header["Content-Md5"], check.DeepEquals, []string{"JvkO/RDWFPEAJS/1bYja2A=="})
}

func readAll(r io.Reader) string {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func (s *S) TestPutAllNoPreviousUpload(c *check.C) {
	// Don't retry the NoSuchUpload error.
	s.DisableRetries()

	etag1 := map[string]string{"ETag": `"etag1"`}
	etag2 := map[string]string{"ETag": `"etag2"`}
	etag3 := map[string]string{"ETag": `"etag3"`}
	testServer.Response(200, nil, InitMultiResultDump)
	testServer.Response(404, nil, NoSuchUploadErrorDump)
	testServer.Response(200, etag1, "")
	testServer.Response(200, etag2, "")
	testServer.Response(200, etag3, "")

	b := s.s3.Bucket("sample")

	multi, err := b.InitMulti("multi", "text/plain", s3.Private)
	c.Assert(err, check.IsNil)

	parts, err := multi.PutAll(strings.NewReader("part1part2last"), 5)
	c.Assert(parts, check.HasLen, 3)
	c.Assert(parts[0].ETag, check.Equals, `"etag1"`)
	c.Assert(parts[1].ETag, check.Equals, `"etag2"`)
	c.Assert(parts[2].ETag, check.Equals, `"etag3"`)
	c.Assert(err, check.IsNil)

	// Init
	testServer.WaitRequest()

	// List old parts. Won't find anything.
	req := testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "GET")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")

	// Send part 1.
	req = testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "PUT")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	c.Assert(req.Form["partNumber"], check.DeepEquals, []string{"1"})
	c.Assert(req.Header["Content-Length"], check.DeepEquals, []string{"5"})
	c.Assert(readAll(req.Body), check.Equals, "part1")

	// Send part 2.
	req = testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "PUT")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	c.Assert(req.Form["partNumber"], check.DeepEquals, []string{"2"})
	c.Assert(req.Header["Content-Length"], check.DeepEquals, []string{"5"})
	c.Assert(readAll(req.Body), check.Equals, "part2")

	// Send part 3 with shorter body.
	req = testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "PUT")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	c.Assert(req.Form["partNumber"], check.DeepEquals, []string{"3"})
	c.Assert(req.Header["Content-Length"], check.DeepEquals, []string{"4"})
	c.Assert(readAll(req.Body), check.Equals, "last")
}

func (s *S) TestPutAllZeroSizeFile(c *check.C) {
	// Don't retry the NoSuchUpload error.
	s.DisableRetries()

	etag1 := map[string]string{"ETag": `"etag1"`}
	testServer.Response(200, nil, InitMultiResultDump)
	testServer.Response(404, nil, NoSuchUploadErrorDump)
	testServer.Response(200, etag1, "")

	b := s.s3.Bucket("sample")

	multi, err := b.InitMulti("multi", "text/plain", s3.Private)
	c.Assert(err, check.IsNil)

	// Must send at least one part, so that completing it will work.
	parts, err := multi.PutAll(strings.NewReader(""), 5)
	c.Assert(parts, check.HasLen, 1)
	c.Assert(parts[0].ETag, check.Equals, `"etag1"`)
	c.Assert(err, check.IsNil)

	// Init
	testServer.WaitRequest()

	// List old parts. Won't find anything.
	req := testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "GET")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")

	// Send empty part.
	req = testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "PUT")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	c.Assert(req.Form["partNumber"], check.DeepEquals, []string{"1"})
	c.Assert(req.Header["Content-Length"], check.DeepEquals, []string{"0"})
	c.Assert(readAll(req.Body), check.Equals, "")
}

func (s *S) TestPutAllResume(c *check.C) {
	etag2 := map[string]string{"ETag": `"etag2"`}
	testServer.Response(200, nil, InitMultiResultDump)
	testServer.Response(200, nil, ListPartsResultDump1)
	testServer.Response(200, nil, ListPartsResultDump2)
	testServer.Response(200, etag2, "")

	b := s.s3.Bucket("sample")

	multi, err := b.InitMulti("multi", "text/plain", s3.Private)
	c.Assert(err, check.IsNil)

	// "part1" and "part3" match the checksums in ResultDump1.
	// The middle one is a mismatch (it refers to "part2").
	parts, err := multi.PutAll(strings.NewReader("part1partXpart3"), 5)
	c.Assert(parts, check.HasLen, 3)
	c.Assert(parts[0].N, check.Equals, 1)
	c.Assert(parts[0].Size, check.Equals, int64(5))
	c.Assert(parts[0].ETag, check.Equals, `"ffc88b4ca90a355f8ddba6b2c3b2af5c"`)
	c.Assert(parts[1].N, check.Equals, 2)
	c.Assert(parts[1].Size, check.Equals, int64(5))
	c.Assert(parts[1].ETag, check.Equals, `"etag2"`)
	c.Assert(parts[2].N, check.Equals, 3)
	c.Assert(parts[2].Size, check.Equals, int64(5))
	c.Assert(parts[2].ETag, check.Equals, `"49dcd91231f801159e893fb5c6674985"`)
	c.Assert(err, check.IsNil)

	// Init
	testServer.WaitRequest()

	// List old parts, broken in two requests.
	for i := 0; i < 2; i++ {
		req := testServer.WaitRequest()
		c.Assert(req.Method, check.Equals, "GET")
		c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	}

	// Send part 2, as it didn't match the checksum.
	req := testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "PUT")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	c.Assert(req.Form["partNumber"], check.DeepEquals, []string{"2"})
	c.Assert(req.Header["Content-Length"], check.DeepEquals, []string{"5"})
	c.Assert(readAll(req.Body), check.Equals, "partX")
}

func (s *S) TestMultiComplete(c *check.C) {
	testServer.Response(200, nil, InitMultiResultDump)
	// Note the 200 response. Completing will hold the connection on some
	// kind of long poll, and may return a late error even after a 200.
	testServer.Response(200, nil, InternalErrorDump)
	testServer.Response(200, nil, "")

	b := s.s3.Bucket("sample")

	multi, err := b.InitMulti("multi", "text/plain", s3.Private)
	c.Assert(err, check.IsNil)

	err = multi.Complete([]s3.Part{{2, `"ETag2"`, 32}, {1, `"ETag1"`, 64}})
	c.Assert(err, check.IsNil)

	testServer.WaitRequest()
	req := testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "POST")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	c.Assert(req.Form.Get("uploadId"), check.Matches, "JNbR_[A-Za-z0-9.]+QQ--")

	var payload struct {
		XMLName xml.Name
		Part    []struct {
			PartNumber int
			ETag       string
		}
	}

	dec := xml.NewDecoder(req.Body)
	err = dec.Decode(&payload)
	c.Assert(err, check.IsNil)

	c.Assert(payload.XMLName.Local, check.Equals, "CompleteMultipartUpload")
	c.Assert(len(payload.Part), check.Equals, 2)
	c.Assert(payload.Part[0].PartNumber, check.Equals, 1)
	c.Assert(payload.Part[0].ETag, check.Equals, `"ETag1"`)
	c.Assert(payload.Part[1].PartNumber, check.Equals, 2)
	c.Assert(payload.Part[1].ETag, check.Equals, `"ETag2"`)
}

func (s *S) TestMultiAbort(c *check.C) {
	testServer.Response(200, nil, InitMultiResultDump)
	testServer.Response(200, nil, "")

	b := s.s3.Bucket("sample")

	multi, err := b.InitMulti("multi", "text/plain", s3.Private)
	c.Assert(err, check.IsNil)

	err = multi.Abort()
	c.Assert(err, check.IsNil)

	testServer.WaitRequest()
	req := testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "DELETE")
	c.Assert(req.URL.Path, check.Equals, "/sample/multi")
	c.Assert(req.Form.Get("uploadId"), check.Matches, "JNbR_[A-Za-z0-9.]+QQ--")
}

func (s *S) TestListMulti(c *check.C) {
	testServer.Response(200, nil, ListMultiResultDump)

	b := s.s3.Bucket("sample")

	multis, prefixes, err := b.ListMulti("", "/")
	c.Assert(err, check.IsNil)
	c.Assert(prefixes, check.DeepEquals, []string{"a/", "b/"})
	c.Assert(multis, check.HasLen, 2)
	c.Assert(multis[0].Key, check.Equals, "multi1")
	c.Assert(multis[0].UploadId, check.Equals, "iUVug89pPvSswrikD")
	c.Assert(multis[1].Key, check.Equals, "multi2")
	c.Assert(multis[1].UploadId, check.Equals, "DkirwsSvPp98guVUi")

	req := testServer.WaitRequest()
	c.Assert(req.Method, check.Equals, "GET")
	c.Assert(req.URL.Path, check.Equals, "/sample/")
	c.Assert(req.Form["uploads"], check.DeepEquals, []string{""})
	c.Assert(req.Form["prefix"], check.DeepEquals, []string{""})
	c.Assert(req.Form["delimiter"], check.DeepEquals, []string{"/"})
	c.Assert(req.Form["max-uploads"], check.DeepEquals, []string{"1000"})
}
