package mail_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/quotedprintable"
	"strings"
	"testing"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"golang.org/x/text/encoding/charmap"
)

func ExampleReader() {
	// Let's assume r is an io.Reader that contains a mail.
	var r io.Reader

	// Create a new mail reader
	mr, err := mail.CreateReader(r)
	if err != nil {
		log.Fatal(err)
	}

	// Read each mail's part
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			b, _ := ioutil.ReadAll(p.Body)
			log.Printf("Got text: %v\n", string(b))
		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			log.Printf("Got attachment: %v\n", filename)
		}
	}
}

func testReader(t *testing.T, r io.Reader) {
	mr, err := mail.CreateReader(r)
	if err != nil {
		t.Fatalf("mail.CreateReader(r) = %v", err)
	}
	defer mr.Close()

	wantSubject := "Your Name"
	subject, err := mr.Header.Subject()
	if err != nil {
		t.Errorf("mr.Header.Subject() = %v", err)
	} else if subject != wantSubject {
		t.Errorf("mr.Header.Subject() = %v, want %v", subject, wantSubject)
	}

	i := 0
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}

		var expectedBody string
		switch i {
		case 0:
			h, ok := p.Header.(*mail.InlineHeader)
			if !ok {
				t.Fatalf("Expected a InlineHeader, but got a %T", p.Header)
			}

			if mediaType, _, _ := h.ContentType(); mediaType != "text/plain" {
				t.Errorf("Expected a plaintext part, not an HTML part")
			}

			expectedBody = "Who are you?"
		case 1:
			h, ok := p.Header.(*mail.AttachmentHeader)
			if !ok {
				t.Fatalf("Expected an AttachmentHeader, but got a %T", p.Header)
			}

			if filename, err := h.Filename(); err != nil {
				t.Error("Expected no error while parsing filename, but got:", err)
			} else if filename != "note.txt" {
				t.Errorf("Expected filename to be %q but got %q", "note.txt", filename)
			}

			expectedBody = "I'm Mitsuha."
		}

		if b, err := ioutil.ReadAll(p.Body); err != nil {
			t.Error("Expected no error while reading part body, but got:", err)
		} else if string(b) != expectedBody {
			t.Errorf("Expected part body to be:\n%v\nbut got:\n%v", expectedBody, string(b))
		}

		i++
	}

	if i != 2 {
		t.Errorf("Expected exactly two parts but got %v", i)
	}
}

func TestReader(t *testing.T) {
	testReader(t, strings.NewReader(mailString))
}

func TestReader_nonMultipart(t *testing.T) {
	s := "Subject: Your Name\r\n" +
		"\r\n" +
		"Who are you?"

	mr, err := mail.CreateReader(strings.NewReader(s))
	if err != nil {
		t.Fatal("Expected no error while creating reader, got:", err)
	}
	defer mr.Close()

	p, err := mr.NextPart()
	if err != nil {
		t.Fatal("Expected no error while reading part, got:", err)
	}

	if _, ok := p.Header.(*mail.InlineHeader); !ok {
		t.Fatalf("Expected a InlineHeader, but got a %T", p.Header)
	}

	expectedBody := "Who are you?"
	if b, err := ioutil.ReadAll(p.Body); err != nil {
		t.Error("Expected no error while reading part body, but got:", err)
	} else if string(b) != expectedBody {
		t.Errorf("Expected part body to be:\n%v\nbut got:\n%v", expectedBody, string(b))
	}

	if _, err := mr.NextPart(); err != io.EOF {
		t.Fatal("Expected io.EOF while reading part, but got:", err)
	}
}

func TestReader_closeImmediately(t *testing.T) {
	s := "Content-Type: text/plain\r\n" +
		"\r\n" +
		"Who are you?"

	mr, err := mail.CreateReader(strings.NewReader(s))
	if err != nil {
		t.Fatal("Expected no error while creating reader, got:", err)
	}

	mr.Close()

	if _, err := mr.NextPart(); err != io.EOF {
		t.Fatal("Expected io.EOF while reading part, but got:", err)
	}
}

func TestReader_nested(t *testing.T) {
	r := strings.NewReader(nestedMailString)

	mr, err := mail.CreateReader(r)
	if err != nil {
		t.Fatalf("mail.CreateReader(r) = %v", err)
	}
	defer mr.Close()

	i := 0
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}

		switch i {
		case 0:
			_, ok := p.Header.(*mail.InlineHeader)
			if !ok {
				t.Fatalf("Expected a InlineHeader, but got a %T", p.Header)
			}

			expectedBody := "I forgot."
			if b, err := ioutil.ReadAll(p.Body); err != nil {
				t.Error("Expected no error while reading part body, but got:", err)
			} else if string(b) != expectedBody {
				t.Errorf("Expected part body to be:\n%v\nbut got:\n%v", expectedBody, string(b))
			}
		case 1:
			_, ok := p.Header.(*mail.AttachmentHeader)
			if !ok {
				t.Fatalf("Expected an AttachmentHeader, but got a %T", p.Header)
			}

			testReader(t, p.Body)
		}

		i++
	}
}

func TestRead_NoDecodeTextAttachment(t *testing.T) {
	const attachmentContentType = "text/plain; charset=windows-1251"
	const attachmentBodyUtf8 = "А-ЯЁа-яё№"
	attachmentBodyRaw, err := charmap.Windows1251.NewEncoder().String(attachmentBodyUtf8)
	if err != nil {
		t.Fatal("encode attach body", err)
	}
	if len([]byte(attachmentBodyRaw)) != 9 {
		t.Fatal("incorrect encode:", []byte(attachmentBodyRaw))
	}
	attachmentBodyQP := new(strings.Builder)
	qpw := quotedprintable.NewWriter(attachmentBodyQP)
	qpw.Write([]byte(attachmentBodyRaw))
	qpw.Close()
	if attachmentBodyQP.Len() != 23 {
		t.Fatal("unexpected QP:", attachmentBodyQP.String())
	}

	inputMiltipart := "Mime-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=IMTHEBOUNDARY\r\n" +
		"\r\n" +
		"--IMTHEBOUNDARY\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Text part\r\n" +
		"--IMTHEBOUNDARY\r\n" +
		"Content-Disposition: attachment; filename=cp1251.txt\r\n" +
		"Content-Type: " + attachmentContentType + "\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" +
		attachmentBodyQP.String() + "\r\n" +
		"--IMTHEBOUNDARY--\r\n"

	for _, option := range []bool{true, false} {
		t.Run(fmt.Sprintf("DecodeTextAttachments = %v", option), func(t *testing.T) {
			optOrig := message.DecodeTextAttachments
			message.DecodeTextAttachments = option
			defer func() {
				message.DecodeTextAttachments = optOrig
			}()

			mr, err := mail.CreateReader(strings.NewReader(inputMiltipart))
			if err != nil {
				t.Fatal("unexpected error: ", err)
			}
			defer mr.Close()

			i := 0
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				} else if err != nil {
					t.Fatal("Expected no error while reading multipart entity, got", err)
				}

				var expectedType string
				var expectedBody string
				switch i {
				case 0:
					expectedType = "text/plain"
					expectedBody = "Text part"
				case 1:
					expectedType = attachmentContentType
					if option {
						expectedBody = attachmentBodyUtf8
					} else {
						expectedBody = attachmentBodyRaw
					}
				}

				if mediaType := p.Header.Get("Content-Type"); mediaType != expectedType {
					t.Errorf("Expected part Content-Type to be %q, got %q", expectedType, mediaType)
				}
				if b, err := ioutil.ReadAll(p.Body); err != nil {
					t.Error("Expected no error while reading part body, got", err)
				} else if s := string(b); s != expectedBody {
					t.Errorf("Expected %q as part body but got %q", expectedBody, s)
				}

				i++
			}

			if i != 2 {
				t.Fatalf("Expected multipart entity to contain exactly 2 parts, got %v", i)
			}
		})
	}
}
