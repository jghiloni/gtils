package persistence_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/pivotalservices/gtils/command"
	"github.com/pivotalservices/gtils/mock"
	"github.com/pivotalservices/gtils/osutils"
	. "github.com/jghiloni/gtils/persistence"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	pgCatchCommand string
)

var _ = Describe("PgDump", func() {

	var (
		pgDumpInstance *PgDump
		ip             string = "0.0.0.0"
		username       string = "testuser"
		password       string = "testpass"
		writer         bytes.Buffer
	)
	Context("Import", func() {
		var (
			localFilePath string
			dir           string
			sftpFailErr   error = errors.New("failed to make sftp connection")
		)

		BeforeEach(func() {
			dir, _ = ioutil.TempDir("", "spec")
			localFilePath = path.Join(dir, "lfile")

			pgDumpInstance = &PgDump{
				IP:       ip,
				Username: username,
				Password: password,
				Caller:   &MockSuccessCall{},
			}
		})

		AfterEach(func() {
			os.RemoveAll(dir)
		})

		Context("called w/ successful sftp connection", func() {
			var output bytes.Buffer
			BeforeEach(func() {
				pgDumpInstance.RemoteOps = &mockRemoteOps{
					Writer: &output,
				}
			})

			It("should copy local file to remote file and return nil error", func() {
				controlString := "hello there"
				l, _ := osutils.SafeCreate(localFilePath)
				l.WriteString(controlString)
				l.Close()
				l, _ = os.Open(localFilePath)
				err := pgDumpInstance.Import(l)
				l.Close()
				lf, _ := os.Open(localFilePath)
				defer lf.Close()
				larray, _ := ioutil.ReadAll(lf)
				Ω(err).Should(BeNil())
				Ω(output.String()).Should(Equal(string(larray[:])))
			})
		})

		Context("called w/ failed sftp connection", func() {
			var output bytes.Buffer
			BeforeEach(func() {
				pgDumpInstance.RemoteOps = &mockRemoteOps{
					Err:    sftpFailErr,
					Writer: &output,
				}
			})

			It("should return sftp connection error", func() {
				controlString := "hello there"
				l, _ := osutils.SafeCreate(localFilePath)
				l.WriteString(controlString)
				l.Close()
				l, _ = os.Open(localFilePath)
				err := pgDumpInstance.Import(l)
				l.Close()
				lf, _ := os.Open(localFilePath)
				defer lf.Close()
				larray, _ := ioutil.ReadAll(lf)

				Ω(err).ShouldNot(BeNil())
				Ω(err).Should(Equal(sftpFailErr))
				Ω(output.String()).ShouldNot(Equal(string(larray[:])))
			})
		})

		Context("called w/ failed copy to remote", func() {
			BeforeEach(func() {
				pgDumpInstance.RemoteOps = &mockRemoteOps{
					Err: mock.ErrReadFailure,
				}
			})

			It("should return failed copy error", func() {
				l := mock.NewReadWriteCloser(mock.ErrReadFailure, nil, nil)
				err := pgDumpInstance.Import(l)
				Ω(err).ShouldNot(BeNil())
				Ω(err).Should(Equal(mock.ErrReadFailure))
			})
		})

		Context("remote call w/ failed result from first call", func() {
			BeforeEach(func() {
				pgDumpInstance.Caller = &MockFailCall{}
				pgDumpInstance.RemoteOps = &mockRemoteOps{}
			})

			It("should return a call error", func() {
				l := mock.NewReadWriteCloser(nil, nil, nil)
				err := pgDumpInstance.Import(l)
				Ω(err).ShouldNot(BeNil())
			})
		})
	})

	Context("Restore", func() {
		var (
			localFilePath string
			dir           string
			output        bytes.Buffer
		)

		Context("Restore Commands", func() {
			BeforeEach(func() {
				dir, _ = ioutil.TempDir("", "spec")
				localFilePath = path.Join(dir, "lfile")

				pgDumpInstance = &PgDump{
					IP:       ip,
					Username: username,
					Password: password,
					Port:     5432,
					Database: "bosh",
					Caller:   &MockSuccessCall{},
					RemoteOps: &mockRemoteOps{
						Writer: &output,
					},
				}
				pgCatchCommand = ""
			})

			AfterEach(func() {
				pgDumpInstance = nil
			})

			It("Should execute the pg command without recreate", func() {
				controlString := "hello there"
				l, _ := osutils.SafeCreate(localFilePath)
				l.WriteString(controlString)
				l.Close()
				l, _ = os.Open(localFilePath)

				rescueStdout := os.Stdout
				r, w, _ := os.Pipe()
				os.Stdout = w

				pgDumpInstance.RecreateOnRestore = false
				pgDumpInstance.Import(l)

				w.Close()
				outBytes, _ := ioutil.ReadAll(r)
				out := string(outBytes)
				os.Stdout = rescueStdout

				cmd := fmt.Sprintf("PGPASSWORD=%s %s -h %s -U %s -x -p %d -c  -d %s %s", password, PGDmpRestoreBin, ip, username, 5432, "bosh", "/tmp/archive.backup")
				Ω(out).Should(Equal(cmd))
			})

			It("Should execute the pg command with recreate", func() {
				controlString := "hello there"
				l, _ := osutils.SafeCreate(localFilePath)
				l.WriteString(controlString)
				l.Close()
				l, _ = os.Open(localFilePath)

				rescueStdout := os.Stdout
			  r, w, _ := os.Pipe()
			  os.Stdout = w

				pgDumpInstance.RecreateOnRestore = true
				pgDumpInstance.Import(l)

			  w.Close()
			  outBytes, _ := ioutil.ReadAll(r)
				out := string(outBytes)
			  os.Stdout = rescueStdout

				cmd := fmt.Sprintf("PGPASSWORD=%s %s -h %s -U %s -x -p %d -c -C -d %s %s", password, PGDmpRestoreBin, ip, username, 5432, "bosh", "/tmp/archive.backup")
				Ω(out).Should(Equal(cmd))
			})
		})
	})

	Context("Dump", func() {
		Context("With caller successfully execute the command", func() {
			BeforeEach(func() {
				pgDumpInstance = &PgDump{
					IP:       ip,
					Username: username,
					Password: password,
					Caller:   &MockSuccessCall{},
				}
				pgCatchCommand = ""
			})

			AfterEach(func() {
				pgDumpInstance = nil
			})

			It("Should execute the pg command", func() {
				var b bytes.Buffer
				pgDumpInstance.Dump(&b)
				cmd := fmt.Sprintf("PGPASSWORD=%s %s -Fc -h %s -U %s -p 0 ", password, PGDmpDumpBin, ip, username)
				Ω(b.String()).Should(Equal(cmd))
			})

			It("Should return nil error", func() {
				err := pgDumpInstance.Dump(&writer)
				Ω(err).Should(BeNil())
			})
		})

		Context("With caller failed to execute command", func() {
			BeforeEach(func() {
				pgDumpInstance = &PgDump{
					IP:       ip,
					Username: username,
					Password: password,
					Caller:   &MockFailCall{},
				}
			})

			AfterEach(func() {
				pgDumpInstance = nil
			})

			It("Should return non nil error", func() {
				err := pgDumpInstance.Dump(&writer)
				Ω(err).ShouldNot(BeNil())
			})
		})
	})
	Context("Constructor tests", func() {
		var sshConfig command.SshConfig
		var err error
		BeforeEach(func() {
			sshConfig = command.SshConfig{
				Username: "userId",
				Password: "password",
				Host:     "127.0.0.1",
				Port:     22,
			}
		})
		Context("NewRemotePgDump", func() {
			Context("With valid config", func() {
				It("Should return non nil PgDump", func() {
					pgDumpInstance, err = NewPgRemoteDump(0, "", "userName", "password", sshConfig)
					Ω(err).Should(BeNil())
					Ω(pgDumpInstance).ShouldNot(BeNil())
				})
			})
		})
		Context("NewRemoteMysqlDumpWithPath", func() {
			Context("With valid config and non-empty path", func() {
				It("Should return non nil MysqlDump", func() {
					pgDumpInstance, err = NewPgRemoteDumpWithPath(0, "", "userName", "password", sshConfig, "/var/somepath")
					Ω(err).Should(BeNil())
					Ω(pgDumpInstance).ShouldNot(BeNil())
				})
			})
			Context("With valid config and empty path", func() {
				It("Should return non nil MysqlDump", func() {
					pgDumpInstance, err = NewPgRemoteDumpWithPath(0, "", "userName", "password", sshConfig, "")
					Ω(err).Should(BeNil())
					Ω(pgDumpInstance).ShouldNot(BeNil())
				})
			})

		})
	})
})
