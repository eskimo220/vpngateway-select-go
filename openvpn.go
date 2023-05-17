package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	configFilePath      = "./20230516/4---219.100.37.220.ovpn"
	openvpnExecutable   = "openvpn"
	ifconfigURL         = "http://ifconfig.me"
	testURL             = "https://www.google.com"
	managementIP        = "127.0.0.1"
	managementPort      = 7505
	connectionTimeout   = 60 * time.Second
	checkInterval       = 1 * time.Second
	dnsUpdateShellPath1 = "/etc/openvpn/update-resolv-conf"
	dnsUpdateShellPath2 = "/etc/openvpn/up.sh"
)

func connect_and_check(configFilePath string) {
	originalOutboundIP := getOutboundIP(ifconfigURL)
	fmt.Printf("Original outbound IP: %s\n", originalOutboundIP)
	originalOutboundIP = getOutboundIP(ifconfigURL)
	fmt.Printf("Original outbound IP: %s\n", originalOutboundIP)
	originalOutboundIP = getOutboundIP(ifconfigURL)
	fmt.Printf("Original outbound IP: %s\n", originalOutboundIP)

	var cmd *exec.Cmd
	dnsShellPath := ""
	if _, err := os.Stat(dnsUpdateShellPath1); err == nil {
		dnsShellPath = dnsUpdateShellPath1
	} else if _, err := os.Stat(dnsUpdateShellPath2); err == nil {
		dnsShellPath = dnsUpdateShellPath2
	} else {
		log.Fatalf("Can not find dns shell.")
	}

	stdoutLogger := log.New(os.Stdout, "OpenVPN stdout: ", log.LstdFlags)
	stderrLogger := log.New(os.Stderr, "OpenVPN stderr: ", log.LstdFlags)

	cmd = exec.Command(openvpnExecutable,
		"--config", configFilePath,
		"--management", managementIP, fmt.Sprintf("%d", managementPort),
		"--script-security", "2",
		"--up", dnsShellPath,
		"--down", dnsShellPath)

	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			stdoutLogger.Println(scanner.Text())
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			stderrLogger.Println(scanner.Text())
		}
	}()

	err := cmd.Start()
	if err != nil {
		log.Fatalf("Failed to start openvpn: %v", err)
	}

	// defer cmd.Process.Signal(syscall.SIGTERM)

	if !waitForVPNConnection(managementIP, managementPort, connectionTimeout) {
		log.Fatal("Cannot establish VPN connection, terminating program")
		return
	}

	vpnOutboundIP := getOutboundIP(ifconfigURL)
	fmt.Printf("Outbound IP after VPN connection: %s\n", vpnOutboundIP)

	if vpnOutboundIP == originalOutboundIP {
		log.Fatal("VPN connection failed, outbound IP did not change")
	}

	if !checkURLConnectivity(testURL) {
		log.Fatal("VPN connection failed, cannot access test URL")
	}
	fmt.Println("VPN connection established successfully")
}

func getOutboundIP(checkURL string) string {

	_, err := net.LookupIP("ifconfig.me")
	if err != nil {
		log.Fatalf("Failed to resolve %v", err)
	}

	resp, err := http.Get(checkURL)
	if err != nil {
		log.Fatalf("Failed to get outbound IP: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
	}

	return string(body)
}

func checkURLConnectivity(url string) bool {
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}
	return true
}

func waitForVPNConnection(ip string, port int, timeout time.Duration) bool {
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 2*time.Second)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		_, err = conn.Write([]byte("state\n"))
		if err != nil {
			log.Printf("Error writing to socket: %v", err)
			conn.Close()
			time.Sleep(1 * time.Second)
			continue
		}

		response := make([]byte, 4096)
		_, err = conn.Read(response)
		if err != nil && err != io.EOF {
			log.Printf("Error reading from socket: %v", err)
			conn.Close()
			time.Sleep(1 * time.Second)
			continue
		}

		log.Println(string(response))

		if strings.Contains(string(response), "CONNECTED,SUCCESS") {
			time.Sleep(20 * time.Second)
			log.Println("VPN connection established")
			conn.Close()
			return true
		}

		conn.Close()
		log.Println("retry")
		time.Sleep(1 * time.Second)
	}

	log.Println("Waiting for VPN connection timed out")
	return false
}
