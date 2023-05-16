package main

import (
	"encoding/base64"
	"encoding/csv"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type ServerData struct {
	CountryShort string
	Score        int
	IP           string
	ConfigData   string
}

func downloadCSV(url, dirName string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	csvFile := filepath.Join(dirName, "vpn_servers.csv")
	err = ioutil.WriteFile(csvFile, body, 0644)
	if err != nil {
		return "", err
	}
	return csvFile, nil
}

func filterAndSortData(csvFile string) ([]ServerData, error) {
	file, err := os.Open(csvFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	// skip one row
	_, err = reader.Read()
	if err != nil {
		return nil, err
	}

	// Read the header line
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	// Get indices based on header line
	countryIndex := findIndex(header, "CountryShort")
	scoreIndex := findIndex(header, "Score")
	ipIndex := findIndex(header, "IP")
	configIndex := findIndex(header, "OpenVPN_ConfigData_Base64")

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var data []ServerData
	for _, record := range records {
		// Skip if the record is too short
		if len(record) <= scoreIndex || len(record) <= countryIndex || len(record) <= ipIndex || len(record) <= configIndex {
			continue
		}
		score, _ := strconv.Atoi(record[scoreIndex])
		if record[countryIndex] == "JP" || record[countryIndex] == "US" {
			data = append(data, ServerData{
				CountryShort: record[countryIndex],
				Score:        score,
				IP:           record[ipIndex],
				ConfigData:   record[configIndex],
			})
		}
	}

	sort.Slice(data, func(i, j int) bool {
		return data[i].Score > data[j].Score
	})

	return data, nil
}

func findIndex(header []string, name string) int {
	for i, v := range header {
		if v == name {
			return i
		}
	}
	return -1
}

func printAndSaveData(data []ServerData, dirName string) error {
	for i, server := range data {
		log.Println(server.CountryShort, server.Score, server.IP)

		fileName := filepath.Join(dirName, strconv.Itoa(i)+"---"+server.IP+".ovpn")
		decodedData, err := base64.StdEncoding.DecodeString(server.ConfigData)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(fileName, decodedData, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	dirName := time.Now().Format("20060102")

	if _, err := os.Stat(dirName); !os.IsNotExist(err) {
		os.RemoveAll(dirName)
		log.Printf("Removed existing directory: %s\n", dirName)
	}

	err := os.MkdirAll(dirName, 0755)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Created new directory: %s\n", dirName)

	url := "http://www.vpngate.net/api/iphone/"
	csvFile, err := downloadCSV(url, dirName)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Downloaded CSV file: %s\n", csvFile)

	data, err := filterAndSortData(csvFile)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Filtered and sorted data")

	err = printAndSaveData(data, dirName)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Printed and saved datas .ovpn files")
}
