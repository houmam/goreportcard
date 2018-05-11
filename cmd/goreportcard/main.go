package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gojp/goreportcard/check"
	"github.com/gojp/goreportcard/handlers"
)

var allScores []handlers.Score

type arrayFlags []string

var checkFlags arrayFlags

func (i *arrayFlags) String() string {
	return fmt.Sprintf("%s", *i)
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	flag.Var(&checkFlags, "check", "Check Params.")
	flag.Parse()

	checkMap := map[string]float64{}

	for _, f := range checkFlags {
		split := strings.Split(f, "=")
		ps, err := strconv.ParseFloat(split[1], 64)

		if err != nil {
			panic(fmt.Sprintf("%v is not a float (%s)", split[1], err.Error()))
		}

		checkMap[split[0]] = ps
	}

	dir := "."
	filenames, skipped, err := check.GoFiles(dir)
	if err != nil {
		log.Fatalf("could not get filenames: %v", err)
	}
	if len(filenames) == 0 {
		log.Fatalf("no .go files found")
	}

	err = check.RenameFiles(skipped)
	if err != nil {
		log.Println("Could not remove files:", err)
	}
	defer check.RevertFiles(skipped)

	checks := []check.Check{
		check.GoFmt{Dir: dir, Filenames: filenames},
		check.GoVet{Dir: dir, Filenames: filenames},
		check.GoLint{Dir: dir, Filenames: filenames},
		check.GoCyclo{Dir: dir, Filenames: filenames},
		check.License{Dir: dir, Filenames: []string{}},
		check.Misspell{Dir: dir, Filenames: filenames},
		check.IneffAssign{Dir: dir, Filenames: filenames},
	}

	ch := make(chan handlers.Score)
	for _, c := range checks {
		go func(c check.Check) {
			p, summaries, err := c.Percentage()
			errMsg := ""
			if err != nil {
				log.Printf("ERROR: (%s) %v", c.Name(), err)
				errMsg = err.Error()
			}
			s := handlers.Score{
				Name:          c.Name(),
				Description:   c.Description(),
				FileSummaries: summaries,
				Weight:        c.Weight(),
				Percentage:    p,
				Error:         errMsg,
			}
			ch <- s
		}(c)
	}

	var (
		total       float64
		totalWeight float64
	)
	for i := 0; i < len(checks); i++ {
		s := <-ch
		allScores = append(allScores, s)
		total += s.Percentage * s.Weight
		totalWeight += s.Weight
	}
	total /= totalWeight

	allpass := true
	for _, score := range allScores {
		pass := true

		if score.Percentage*100 < checkMap[score.Name] {
			pass = false
			allpass = false
		}

		fmt.Printf("%s: %.2f%% (>= %.2f%% == %t)\n", score.Name, score.Percentage*100, checkMap[score.Name], pass)

		if !pass {
			fmt.Printf("\nIssues:\n")
			for _, fs := range score.FileSummaries {
				fmt.Printf("\t%s\n", fs.Filename)
				for _, e := range fs.Errors {
					fmt.Printf("\t\tLine Number: %d (%s)\n", e.LineNumber, e.ErrorString)
				}
			}
		}
		fmt.Println()
	}

	if allpass {
		fmt.Println("Passed")
	} else {
		fmt.Println("Failed")
	}
}
