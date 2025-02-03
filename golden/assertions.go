package golden

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"testing"
	"text/template"
)

// Assert compares the actual data received with the expected data in the
// golden files. If the update flag is set, it will also update the golden
// file.
//
// `name` refers to the name of the test and it should typically be unique
// within the package. Also it should be a valid file name (so keeping to
// `a-z0-9\-\_` is a good idea).
func (g *Golden) Assert(t *testing.T, name string, actualData []byte) {
	t.Helper()
	if *update {
		err := g.Update(t, name, actualData)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
	}

	err := g.compare(t, name, actualData)
	if err != nil {
		{
			var e *fixtureNotFoundError
			if errors.As(err, &e) {
				t.Error(err)
				t.FailNow()
				return
			}
		}

		{
			var e *fixtureMismatchError
			if errors.As(err, &e) {
				t.Error(err)
				return
			}
		}

		t.Error(err)
	}
}

// AssertJSON compares the actual json data received with expected data in the
// golden files. If the update flag is set, it will also update the golden
// file.
//
// `name` refers to the name of the test and it should typically be unique
// within the package. Also it should be a valid file name (so keeping to
// `a-z0-9\-\_` is a good idea).
func (g *Golden) AssertJSON(t *testing.T, name string, actualJSONData interface{}) {
	t.Helper()
	js, err := json.MarshalIndent(actualJSONData, "", "  ")

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	g.Assert(t, name, normalizeLF(js))
}

// AssertXML compares the actual xml data received with expected data in the
// golden files. If the update flag is set, it will also update the golden
// file.
//
// `name` refers to the name of the test and it should typically be unique
// within the package. Also it should be a valid file name (so keeping to
// `a-z0-9\-\_` is a good idea).
func (g *Golden) AssertXML(t *testing.T, name string, actualXMLData interface{}) {
	t.Helper()
	x, err := xml.MarshalIndent(actualXMLData, "", "  ")

	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	g.Assert(t, name, normalizeLF(x))
}

// normalizeLF normalizes line feed character set across os (es)
// \r\n (windows) & \r (mac) into \n (unix)
func normalizeLF(d []byte) []byte {
	// if empty / nil return as is
	if len(d) == 0 {
		return d
	}
	// replace CR LF \r\n (windows) with LF \n (unix)
	d = bytes.ReplaceAll(d, []byte{13, 10}, []byte{10})
	// replace CF \r (mac) with LF \n (unix)
	d = bytes.ReplaceAll(d, []byte{13}, []byte{10})
	return d
}

// AssertWithTemplate compares the actual data received with the expected data in the
// golden files after executing it as a template with data parameter. If the
// update flag is set, it will also update the golden file.  `name` refers to
// the name of the test and it should typically be unique within the package.
// Also it should be a valid file name (so keeping to `a-z0-9\-\_` is a good
// idea).
func (g *Golden) AssertWithTemplate(t *testing.T, name string, data interface{}, actualData []byte) {
	t.Helper()
	if *update {
		err := g.Update(t, name, actualData)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
	}

	err := g.compareTemplate(t, name, data, actualData)
	if err != nil {
		{
			var e *fixtureNotFoundError
			if errors.As(err, &e) {
				t.Error(err)
				t.FailNow()
				return
			}
		}

		{
			var e *fixtureMismatchError
			if errors.As(err, &e) {
				t.Error(err)
				return
			}
		}

		t.Error(err)
	}
}

// compare is reading the golden fixture file and compare the stored data with
// the actual data.
func (g *Golden) compare(t *testing.T, name string, actualData []byte) error {
	t.Helper()
	expectedData, err := os.ReadFile(g.GoldenFileName(t, name))

	if err != nil {
		if os.IsNotExist(err) {
			return newErrFixtureNotFound()
		}

		return fmt.Errorf("expected %s to be nil", err.Error())
	}

	if !bytes.Equal(normalizeLF(actualData), normalizeLF(expectedData)) {
		msg := "Result did not match the golden fixture. Diff is below:\n\n"
		actual := string(actualData)
		expected := string(expectedData)

		if g.diffFn != nil {
			msg += g.diffFn(actual, expected)
		} else {
			msg += Diff(g.diffEngine, actual, expected)
		}

		return newErrFixtureMismatch(msg)
	}

	return nil
}

// compareTemplate is reading the golden fixture file and compare the stored
// data with the actual data.
func (g *Golden) compareTemplate(t *testing.T, name string, data interface{}, actualData []byte) error {
	t.Helper()
	expectedDataTmpl, err := os.ReadFile(g.GoldenFileName(t, name))

	if err != nil {
		if os.IsNotExist(err) {
			return newErrFixtureNotFound()
		}

		return fmt.Errorf("expected %s to be nil", err.Error())
	}

	missingKey := "error"
	if g.ignoreTemplateErrors {
		missingKey = "default"
	}

	tmpl, err := template.New("test").Option("missingkey=" + missingKey).Parse(string(expectedDataTmpl))
	if err != nil {
		return fmt.Errorf("expected %s to be nil", err.Error())
	}

	var expectedData bytes.Buffer
	err = tmpl.Execute(&expectedData, data)
	if err != nil {
		return newErrMissingKey("Template error: " + err.Error())
	}

	if !bytes.Equal(actualData, expectedData.Bytes()) {
		msg := "Result did not match the golden fixture. Diff is below:\n\n"
		actual := string(actualData)
		expected := expectedData.String()

		if g.diffFn != nil {
			msg += g.diffFn(actual, expected)
		} else {
			msg += Diff(g.diffEngine, actual, expected)
		}

		return newErrFixtureMismatch(msg)
	}

	return nil
}
