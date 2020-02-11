package main

import (
	"fmt"
	"io/ioutil"
	"strings"

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

	//Json that have to be loaded as template
	if j, err = ioutil.ReadFile("conf/data.json"); err != nil {
		panic("unable to find the file with json data")
	}
	// key of json that have to be modified
	if c, err := ioutil.ReadFile("conf/conf.ini"); err == nil {
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
					if err := ioutil.WriteFile(fmt.Sprintf("./generated/%d", fileName), k, 0644); err != nil {
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
