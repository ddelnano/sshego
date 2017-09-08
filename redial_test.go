package sshego

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
	ssh "github.com/glycerine/sshego/xendor/github.com/glycerine/xcryptossh"
)

func Test050RedialGraphMaintained(t *testing.T) {
	cv.Convey("With AutoReconnect true, our ssh client automatically redials the ssh server if disconnected", t, func() {

		srvCfg, r1 := GenTestConfig()
		cliCfg, r2 := GenTestConfig()

		// now that we have all different ports, we
		// must release them for use below.
		r1()
		r2()
		defer TempDirCleanup(srvCfg.Origdir, srvCfg.Tempdir)
		srvCfg.NewEsshd()
		ctx := context.Background()
		halt := ssh.NewHalter()

		srvCfg.Esshd.Start(ctx)
		// create a new acct
		mylogin := "bob"
		myemail := "bob@example.com"
		fullname := "Bob Fakey McFakester"
		pw := fmt.Sprintf("%x", string(CryptoRandBytes(30)))

		p("srvCfg.HostDb = %#v", srvCfg.HostDb)
		toptPath, qrPath, rsaPath, err := srvCfg.HostDb.AddUser(
			mylogin, myemail, pw, "gosshtun", fullname, "")

		cv.So(err, cv.ShouldBeNil)

		cv.So(strings.HasPrefix(toptPath, srvCfg.Tempdir), cv.ShouldBeTrue)
		cv.So(strings.HasPrefix(qrPath, srvCfg.Tempdir), cv.ShouldBeTrue)
		cv.So(strings.HasPrefix(rsaPath, srvCfg.Tempdir), cv.ShouldBeTrue)

		pp("toptPath = %v", toptPath)
		pp("qrPath = %v", qrPath)
		pp("rsaPath = %v", rsaPath)

		hostport := fmt.Sprintf("%v:%v", srvCfg.SSHdServer.Host, srvCfg.SSHdServer.Port)
		uhp1 := &ssh.UHP{User: mylogin, HostPort: hostport}

		// try to login to esshd

		// need an ssh client

		// allow server to be discovered
		cliCfg.AddIfNotKnown = true
		cliCfg.TestAllowOneshotConnect = true

		totpUrl, err := ioutil.ReadFile(toptPath)
		panicOn(err)
		totp := string(totpUrl)

		// tell the client not to run an esshd
		cliCfg.EmbeddedSSHd.Addr = ""
		//cliCfg.LocalToRemote.Listen.Addr = ""
		//rev := cliCfg.RemoteToLocal.Listen.Addr
		cliCfg.RemoteToLocal.Listen.Addr = ""

		_, netconn, err := cliCfg.SSHConnect(
			ctx,
			cliCfg.KnownHosts,
			mylogin,
			rsaPath,
			srvCfg.EmbeddedSSHd.Host,
			srvCfg.EmbeddedSSHd.Port,
			pw,
			totp,
			halt)

		reconnectSub := cliCfg.ClientReconnectNeededTower.Subscribe()

		// we should be able to login, but then the sshd should
		// reject the port forwarding request.
		//
		// Anyway, forward request denies does indicate we
		// logged in when all three (RSA, TOTP, passphrase)
		// were given.
		pp("err is %#v", err)
		// should have succeeded in logging in
		cv.So(err, cv.ShouldBeNil)

		netconn.Close()
		time.Sleep(5 * time.Second)
		log.Printf("redial test: just after Blinking the connection...")

		dur := 2 * time.Second
		select {
		case <-time.After(dur):
			panic(fmt.Sprintf("redial_test: bad, no reconnect in '%v'", dur))
		case who := <-reconnectSub:
			if ssh.UHPEqual(who, uhp1) {
				log.Printf("redial_test: good, reconnected to '%v'", who)
			} else {
				log.Printf("redial_test: bad, expected reconnect to uhp1, but got reconnected to '%v'.", who)
			}
		}
	})
}
