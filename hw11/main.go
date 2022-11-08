package main

import (
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
)

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}

func dirTree(out io.Writer, path string, printFiles bool) error {
	return dirTreeHelper(out, path, printFiles, "")
}

func dirTreeHelper(out io.Writer, path string, printFiles bool, prefix string) error {
	rawItems, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
	buf := make([]byte, 0, 100)
	var isLastRec bool
	var isDir bool
	var newPrefix string
	items := []os.FileInfo{}
	for _, item := range rawItems {
		if item.Name()[0] == '.' {
			continue
		}
		if !item.IsDir() && !printFiles {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name() < items[j].Name() })

	for i := 0; i < len(items); i++ {
		isLastRec = i == len(items)-1
		itemName := items[i].Name()
		isDir = items[i].IsDir()

		buf = buf[:0]
		buf = append(buf, []byte(prefix)...)
		if isLastRec {
			buf = append(buf, []byte("└───")...)
		} else {
			buf = append(buf, []byte("├───")...)
		}
		buf = append(buf, []byte(itemName)...)

		if !isDir {
			buf = append(buf, ' ', '(')
			if items[i].Size() > 0 {
				buf = append(buf, []byte(strconv.Itoa(int(items[i].Size())))...)
				buf = append(buf, 'b')
			} else {
				buf = append(buf, []byte("empty")...)
			}
			buf = append(buf, ')')
		}
		buf = append(buf, '\n')
		_, err = out.Write(buf)
		if err != nil {
			return err
		}

		if isDir {
			if isLastRec {
				newPrefix = prefix + "\t"
			} else {
				newPrefix = prefix + "│\t"
			}
			err = dirTreeHelper(out, path+string(os.PathSeparator)+itemName, printFiles, newPrefix)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
