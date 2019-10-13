// Package templateutil contains template functions from prometheus_bot
package templateutil

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

/**
 * Subdivideb by 1024
 */
const (
	Kb = iota
	Mb
	Gb
	Tb
	Pb
	Eb
	Zb
	Yb
	InformationSizeMAX
)

/**
 * Subdivided by 10000
 */
const (
	K = iota
	M
	G
	T
	P
	E
	Z
	Y
	ScaleSizeMAX
)

func roundPrec(x float64, prec int) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}

	sign := 1.0
	if x < 0 {
		sign = -1
		x *= -1
	}

	var rounder float64
	pow := math.Pow(10, float64(prec))
	intermed := x * pow
	_, frac := math.Modf(intermed)

	if frac >= 0.5 {
		rounder = math.Ceil(intermed)
	} else {
		rounder = math.Floor(intermed)
	}

	return rounder / pow * sign
}

// StrFormatMeasureUnit formats the template
func StrFormatMeasureUnit(MeasureUnit string, value string, templateSplitToken string) string {
	var RetStr string
	MeasureUnit = strings.TrimSpace(MeasureUnit) // Remove space
	SplittedMUnit := strings.SplitN(MeasureUnit, templateSplitToken, 3)

	Initial := 0
	// If is declared third part of array, then Measure unit start from just scaled measure unit.
	// Example Kg is Kilo g, but all people use Kg not g, then you will put here 3 Kilo. Bot strart convert from here.
	if len(SplittedMUnit) > 2 {
		tmp, err := strconv.ParseInt(SplittedMUnit[2], 10, 8)
		if err != nil {
			log.Println("Could not convert value to int")
			// if !*debug {
			// If is running in production leave daemon live. else here will die with log error.
			// return "" // Break execution and return void string, bot will log somethink
			// }
		}
		Initial = int(tmp)
	}

	switch SplittedMUnit[0] {
	case "kb":
		RetStr = StrFormatByte(value, Initial)
	case "s":
		RetStr = StrFormatScale(value, Initial)
	case "f":
		RetStr = StrFormatFloat(value)
	case "i":
		RetStr = StrFormatInt(value)
	default:
		RetStr = StrFormatInt(value)
	}

	if len(SplittedMUnit) > 1 {
		RetStr += SplittedMUnit[1]
	}

	return RetStr
}

// StrFormatByte scales number for It measure unit
func StrFormatByte(in string, j1 int) string {
	var strSize string

	f, err := strconv.ParseFloat(in, 64)

	if err != nil {
		panic(err)
	}

	for j1 = 0; j1 < (InformationSizeMAX + 1); j1++ {

		if j1 >= InformationSizeMAX {
			strSize = "Yb"
			break
		} else if f > 1024 {
			f /= 1024.0
		} else {

			switch j1 {
			case Kb:
				strSize = "Kb"
			case Mb:
				strSize = "Mb"
			case Gb:
				strSize = "Gb"
			case Tb:
				strSize = "Tb"
			case Pb:
				strSize = "Pb"
			case Eb:
				strSize = "Eb"
			case Zb:
				strSize = "Zb"
			case Yb:
				strSize = "Yb"
			}
			break
		}
	}

	strFl := strconv.FormatFloat(f, 'f', 2, 64)
	return fmt.Sprintf("%s %s", strFl, strSize)
}

// StrFormatScale formats number for fisics measure unit
func StrFormatScale(in string, j1 int) string {
	var strSize string

	f, err := strconv.ParseFloat(in, 64)

	if err != nil {
		panic(err)
	}

	for j1 = 0; j1 < (ScaleSizeMAX + 1); j1++ {

		if j1 >= ScaleSizeMAX {
			strSize = "Y"
			break
		} else if f > 1000 {
			f /= 1000.0
		} else {
			switch j1 {
			case K:
				strSize = "K"
			case M:
				strSize = "M"
			case G:
				strSize = "G"
			case T:
				strSize = "T"
			case P:
				strSize = "P"
			case E:
				strSize = "E"
			case Z:
				strSize = "Z"
			case Y:
				strSize = "Y"
			default:
				strSize = "Y"
			}
			break
		}
	}

	strFl := strconv.FormatFloat(f, 'f', 2, 64)
	return fmt.Sprintf("%s %s", strFl, strSize)
}

// StrFormatInt formats as integer
func StrFormatInt(i string) string {
	v, _ := strconv.ParseInt(i, 10, 64)
	val := strconv.FormatInt(v, 10)
	return val
}

// StrFormatFloat formats as float
func StrFormatFloat(f string) string {
	v, _ := strconv.ParseFloat(f, 64)
	v = roundPrec(v, 2)
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// StrFormatDate formats as date
func StrFormatDate(toformat string, templateTimeZone string, templateTimeOutFormat string) string {

	// Error handling
	if templateTimeZone == "" {
		log.Println("template_time_zone is not set, if you use template and `str_FormatDate` func is required")
		panic(nil)
	}

	if templateTimeOutFormat == "" {
		log.Println("template_time_outdata param is not set, if you use template and `str_FormatDate` func is required")
		panic(nil)
	}

	t, err := time.Parse(time.RFC3339Nano, toformat)

	if err != nil {
		fmt.Println(err)
	}

	loc, _ := time.LoadLocation(templateTimeZone)

	return t.In(loc).Format(templateTimeOutFormat)
}

// HasKey checks if the map contains the key
func HasKey(dict map[string]interface{}, keySearch string) bool {
	if _, ok := dict[keySearch]; ok {
		return true
	}
	return false
}
