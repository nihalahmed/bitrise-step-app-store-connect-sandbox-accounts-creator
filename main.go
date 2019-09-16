package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	email := getEnvVar("app_store_connect_email")
	password := getEnvVar("app_store_connect_password")
	sandboxAccountEmailPrefix := getEnvVar("sandbox_account_email_prefix")
	sandboxAccountPassword := getEnvVar("sandbox_account_password")
	numSandboxAccounts := getEnvVarNumber("number_of_sandbox_accounts")

	_, err := logIn(email, password)
	if err != nil {
		log.Printf("Failed to log in to App Store Connect: %s", err)
		os.Exit(1)
	}

	var accounts []string
	ts := time.Now().Unix()
	for i := 0; i < numSandboxAccounts; i++ {
		sandboxAccountEmail := fmt.Sprintf("%s%d%d@test.com", sandboxAccountEmailPrefix, ts, i)
		id, err := createAccount(sandboxAccountEmail, sandboxAccountPassword)
		if err == nil {
			accounts = append(accounts, fmt.Sprintf("%s|%s|%s", id, sandboxAccountEmail, sandboxAccountPassword))
		} else {
			log.Printf("Failed to create sandbox account: %s", err)
			break
		}
	}

	if len(accounts) != numSandboxAccounts {
		for _, account := range accounts {
			id := strings.Split(account, "|")[0]
			_, err = deleteAccount(id)
			if err != nil {
				log.Printf("Failed to delete sandbox account: %s", err)
			}
		}
		os.Exit(1)
	}

	o, err := exec.Command("bitrise", "envman", "add", "--key", "APP_STORE_CONNECT_SANDBOX_ACCOUNTS", "--value", strings.Join(accounts, ",")).CombinedOutput()
	if err != nil {
		log.Printf("Failed to expose output with envman, error: %s | output: %s", err, o)
		os.Exit(1)
	}
}

func logIn(username, password string) (map[string]interface{}, error) {
	log.Printf("POST request: %s", "https://idmsa.apple.com/appleauth/auth/signin")

	return executeCurl("-X", "POST", "-H", "X-Apple-Widget-Key: e0b80c3bf78523bfe80974d320935bfa30add02e1bff88ec2166c6bd5a706c42", "-H", "Accept: application/json", "-H", "Content-Type: application/json", "-d", fmt.Sprintf("{\"accountName\":\"%s\",\"password\":\"%s\",\"rememberMe\":false}", username, password), "https://idmsa.apple.com/appleauth/auth/signin", "--cookie-jar", "cookie.txt", "-s", "-w", "|%{http_code}")
}

func createAccount(email, password string) (string, error) {
	log.Printf("POST request: %s", "https://appstoreconnect.apple.com/iris/v1/sandboxTesters")

	response, err := executeCurl("-X", "POST", "-H", "Accept: application/vnd.api+json", "-H", "Content-Type: application/vnd.api+json", "-d", fmt.Sprintf("{\"data\":{\"type\":\"sandboxTesters\",\"attributes\":{\"firstName\":\"Automated\",\"lastName\":\"Tester\",\"email\":\"%s\",\"password\":\"%s\",\"confirmPassword\":\"%s\",\"secretQuestion\":\"galaxy\",\"secretAnswer\":\"milky way\",\"appStoreTerritory\":\"CAN\",\"birthDate\":\"1980-01-01\"}}}", email, password, password), "https://appstoreconnect.apple.com/iris/v1/sandboxTesters", "-b", "cookie.txt", "-s", "-w", "|%{http_code}")

	if err != nil {
		return "", err
	}

	if data, ok := response["data"].(map[string]interface{}); ok {
		if id, ok := data["id"].(string); ok {
			return id, nil
		} else {
			return "", errors.New("Key id not found in data")
		}
	} else {
		return "", errors.New("Key data not found in response")
	}
}

func deleteAccount(id string) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://appstoreconnect.apple.com/iris/v1/sandboxTesters/%s", id)

	log.Printf("DELETE request: %s", url)

	return executeCurl("-X", "DELETE", "-H", "Accept: application/vnd.api+json", "-H", "Content-Type: application/vnd.api+json", url, "-b", "cookie.txt", "-s", "-w", "|%{http_code}")
}

func executeCurl(arg ...string) (map[string]interface{}, error) {
	var result map[string]interface{}
	o, err := exec.Command("curl", arg...).CombinedOutput()
	if err != nil {
		return result, err
	}

	log.Printf("Curl output: %s", o)

	components := strings.Split(string(o), "|")
	if len(components) != 2 {
		return result, errors.New("Invalid curl command output")
	}

	res := components[0]
	code, err := strconv.Atoi(components[1])
	if err != nil {
		return result, errors.New("Invalid status code in curl command output")
	}

	if res != "" {
		err = json.NewDecoder(strings.NewReader(res)).Decode(&result)
		if err != nil {
			return result, err
		}	
	}

	if code >= 200 && code <= 299 {
		return result, nil
	} else {
		return result, errors.New(fmt.Sprintf("HTTP status code %d not in the 2xx range", code))
	}
}

func getEnvVar(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Printf("Environment variable %s not set", key)
		os.Exit(1)
	}
	return value
}

func getEnvVarNumber(key string) int {
	value := os.Getenv(key)
	if value == "" {
		log.Printf("Environment variable %s not set", key)
		os.Exit(1)
	}
	numValue, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("Environment variable %s not a number", key)
		os.Exit(1)
	}
	return numValue
}
