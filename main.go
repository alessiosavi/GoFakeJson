package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	fileutils "github.com/alessiosavi/GoGPUtils/files"
	stringutils "github.com/alessiosavi/GoGPUtils/string"

	"github.com/buger/jsonparser"
)

type configuration struct {
	path  string
	value []string
}

func main() {
	var (
		conf     []configuration
		confData []string
		j        []byte
		err      error
	)

	jN, cN, oN := inputParameter()
	//Json that have to be loaded as template
	if j, err = ioutil.ReadFile(jN); err != nil {
		panic("unable to find the file with json data")
	}
	// key of json that have to be modified
	if c, err := ioutil.ReadFile(cN); err == nil {
		// Save the list of the key:value related to the data that have to be substitute
		t := strings.Split(string(c), "\n")
		conf = make([]configuration, len(t))
		for i := range t {
			if strings.Contains(t[i], "=") {
				confData = strings.Split(string(t[i]), "=")
				conf[i].path = confData[0]
				conf[i].value = strings.Split(confData[1], ",")
			} else {
				panic("not a valid key=value(s)")
			}
		}
	} else {
		panic("unable to find the configuration file")
	}
	var jsonDataModifed [][]byte
	jsonDataModifed = append(jsonDataModifed, j)

	var fileName int
	for i := range conf {
		for j := range jsonDataModifed {
			for _, s := range conf[i].value {
				if k, err := jsonparser.Set(jsonDataModifed[j], []byte("\""+s+"\""), strings.Split(conf[i].path, ".")...); err == nil {
					jsonDataModifed = append(jsonDataModifed, k)
					if err := ioutil.WriteFile(fmt.Sprintf("%s/%d.json", oN, fileName), k, 0644); err != nil {
						panic("Error: " + err.Error())
					}
					fileName++
				} else {
					panic("error: " + err.Error())
				}
			}
		}
	}

	// Remove files that are equals
	filesList := fileutils.ListFile(oN)
	var toRemove []string
	for i := 0; i < len(filesList); i++ {
		for j := i + 1; j < len(filesList); j++ {
			if deepCompare(filesList[i], filesList[j]) {
				toRemove = append(toRemove, filesList[i])
				break
			}
		}
	}
	for i := range toRemove {
		deleteFile(toRemove[i])
	}
	fmt.Println("Found equals files: ", toRemove)
}

func deepCompare(file1, file2 string) bool {
	if !fileutils.FileExists(file1) {
		log.Fatal("File [", file1, "] does not exist!")
	}

	if !fileutils.FileExists(file2) {
		log.Fatal("File [", file2, "] does not exist!")
	}

	var size1, size2 int64
	var err error
	var f1, f2 *os.File
	// Get file size of file1
	size1, err = fileutils.GetFileSize(file1)
	if err != nil {
		log.Fatal("Unable to read file [" + file1 + "]")
	}

	// Get file size of file2
	size2, err = fileutils.GetFileSize(file2)
	if err != nil {
		log.Fatal("Unable to read file [" + file2 + "]")
	}

	if size1 != size2 {
		return false
	}

	if f1, err = os.Open(file1); err != nil {
		log.Fatal(err)
	}
	defer f1.Close()

	if f2, err = os.Open(file2); err != nil {
		log.Fatal(err)
	}
	defer f2.Close()

	b1 := make([]byte, chunkSize)
	b2 := make([]byte, chunkSize)
	for {
		_, err1 := f1.Read(b1)
		_, err2 := f2.Read(b2)
		if err1 != nil || err2 != nil {
			if err1 == io.EOF && err2 == io.EOF {
				return true
			} else if err1 == io.EOF || err2 == io.EOF {
				return false
			} else {
				log.Fatal(err1, err2)
			}
		}
		if !bytes.Equal(b1, b2) {
			return false
		}
	}
}

const chunkSize = 1024

func inputParameter() (string, string, string) {
	j := flag.String("json", "./conf/data.json", "The path related to the input json to create")
	c := flag.String("conf", "./conf/conf.ini", "The path related to the input configuration")
	o := flag.String("output", "./generated", "The path related to the input json to create")

	flag.Parse()

	if stringutils.IsBlank(*j) || !fileutils.IsFile(*j) {
		panic("json file not passed in configuration")
	}
	if stringutils.IsBlank(*c) || !fileutils.IsFile(*c) {
		panic("configuration file not passed in configuration")
	}

	if stringutils.IsBlank(*o) {
		panic("output dir not valid")
	}
	if !fileutils.IsDir(*o) {
		if err := fileutils.CreateDir(*o); err != nil {
			panic("Error creating dir [*o]: " + err.Error())
		}
	}
	return *j, *c, *o
}

func deleteFile(path string) {
	var err = os.Remove(path)
	if isError(err) {
		return
	}
}

func isError(err error) bool {
	if err != nil {
		fmt.Println(err.Error())
	}
	return (err != nil)
}
