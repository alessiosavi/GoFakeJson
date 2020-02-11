package main

import (
	"flag"
	"fmt"
	"io/ioutil"
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
					//fmt.Println(string(k))
					if err := ioutil.WriteFile(fmt.Sprintf("%s/%d", oN, fileName), k, 0644); err != nil {
						fmt.Println("Error: ", err)
					}
					fileName++
				} else {
					panic("error: " + err.Error())
				}
			}
		}
	}
}

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
		fileutils.CreateDir(*o)
	}

	return *j, *c, *o
}
