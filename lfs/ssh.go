package lfs

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"io/ioutil"
	"os"

	"github.com/chrissoc/git-lfs/vendor/_nuts/github.com/rubyist/tracerx"
)

type sshAuthResponse struct {
	Message   string            `json:"-"`
	Href      string            `json:"href"`
	Header    map[string]string `json:"header"`
	ExpiresAt string            `json:"expires_at"`
}

// if it is discovered that the values from the cache are not valid,
// they must be cleared at that time.
func sshClearAuthenticateCache(operation string) {
	endpoint := Config.Endpoint()
	
	cacheFileName := "ssh-cache-" + endpoint.SshUserAndHost + "-" + operation
	cacheFileFullPath := filepath.Join(LocalGitDir, "lfs", cacheFileName)
	
	os.Remove(cacheFileFullPath)
}

func sshAuthenticate(endpoint Endpoint, operation, oid string) (sshAuthResponse, error) {

	// This is only used as a fallback where the Git URL is SSH but server doesn't support a full SSH binary protocol
	// and therefore we derive a HTTPS endpoint for binaries instead; but check authentication here via SSH

	res := sshAuthResponse{}
	if len(endpoint.SshUserAndHost) == 0 {
		return res, nil
	}
	
	// attempt to read in the cached version instead of going to the ssh server.
	//var cache [5000]byte
	cacheFileName := "ssh-cache-" + endpoint.SshUserAndHost + "-" + operation
	cacheFileBasePath := filepath.Join(LocalGitDir, "lfs")
	cacheFileFullPath := filepath.Join(cacheFileBasePath, cacheFileName)
	cache, err := ioutil.ReadFile(cacheFileFullPath)
	if err == nil {
		// successfully read in cache, now to parse it
		// Processing result
		if err != nil {
			res.Message = "Failed to load cache file: " + cacheFileName
		} else {
			err = json.Unmarshal(cache, &res)
		}
	} else {
	
		tracerx.Printf("ssh: %s git-lfs-authenticate %s %s %s",
			endpoint.SshUserAndHost, endpoint.SshPath, operation, oid)

		exe, args := sshGetExeAndArgs(endpoint)
		args = append(args,
			"git-lfs-authenticate",
			endpoint.SshPath,
			operation, oid)

		cmd := exec.Command(exe, args...)

		// Save stdout and stderr in separate buffers
		var outbuf, errbuf bytes.Buffer
		cmd.Stdout = &outbuf
		cmd.Stderr = &errbuf

		// Execute command
		err := cmd.Start()
		if err == nil {
			err = cmd.Wait()
		}
		
		// Processing result
		if err != nil {
			res.Message = errbuf.String()
		} else {
			err = json.Unmarshal(outbuf.Bytes(), &res)
		}
		
		// If we got to this side of the else statment then we do not already
		// have a functioning cache file for this user and host.
		// So save off our message in the cache
		os.MkdirAll(cacheFileBasePath, 0777)
		ioutil.WriteFile(cacheFileFullPath, outbuf.Bytes(), 0622)
	}

	return res, err
}

// Return the executable name for ssh on this machine and the base args
// Base args includes port settings, user/host, everything pre the command to execute
func sshGetExeAndArgs(endpoint Endpoint) (exe string, baseargs []string) {
	if len(endpoint.SshUserAndHost) == 0 {
		return "", nil
	}

	isPlink := false
	isTortoise := false

	ssh := Config.Getenv("GIT_SSH")
	if ssh == "" {
		ssh = "ssh"
	} else {
		basessh := filepath.Base(ssh)
		// Strip extension for easier comparison
		if ext := filepath.Ext(basessh); len(ext) > 0 {
			basessh = basessh[:len(basessh)-len(ext)]
		}
		isPlink = strings.EqualFold(basessh, "plink")
		isTortoise = strings.EqualFold(basessh, "tortoiseplink")
	}

	args := make([]string, 0, 4)
	if isTortoise {
		// TortoisePlink requires the -batch argument to behave like ssh/plink
		args = append(args, "-batch")
	}

	if len(endpoint.SshPort) > 0 {
		if isPlink || isTortoise {
			args = append(args, "-P")
		} else {
			args = append(args, "-p")
		}
		args = append(args, endpoint.SshPort)
	}
	args = append(args, endpoint.SshUserAndHost)

	return ssh, args
}
