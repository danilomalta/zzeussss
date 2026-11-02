// titan-fiscal validates offline fiscal inputs and private installation
// configuration. It never accepts inline credentials or transmits documents.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/fiscalproduction"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "titan-fiscal:", err)
		os.Exit(1)
	}
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > fiscalcore.MaxXML {
		return nil, errors.New("input must be a regular file up to 1 MiB")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("input changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(f, fiscalcore.MaxXML+1))
	if err != nil || len(raw) > fiscalcore.MaxXML {
		return nil, errors.New("input too large or unreadable")
	}
	return raw, nil
}
func writeNew(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func decode(raw []byte, target any) error {
	return fiscalcore.DecodeInput(raw, target)
}
func run(args []string, stdout io.Writer) error {
	if len(args) > 0 && strings.HasPrefix(args[0], "vault-") {
		return vaultCommand(args, stdout, privateTerminal)
	}
	if len(args) == 1 && args[0] == "capabilities" {
		return json.NewEncoder(stdout).Encode(fiscalcore.ComponentCapabilities())
	}
	if len(args) < 1 {
		return errors.New("use capabilities, check-production-config, tax-quote, build-nfe, build-dps, build-dps-homolog or validate; offline only")
	}
	fs := flag.NewFlagSet("titan-fiscal", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	input := fs.String("in", "", "input file")
	output := fs.String("out", "", "new output file")
	kind := fs.String("kind", "", "schema kind")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 || *input == "" {
		return errors.New("invalid arguments; --in is required")
	}
	switch args[0] {
	case "check-production-config":
		if *output != "" || *kind != "" {
			return errors.New("check-production-config requires only --in; it never writes or transmits")
		}
		report, err := fiscalproduction.InspectConfiguration(*input)
		if err != nil {
			return fiscalproduction.ErrUnavailable
		}
		return json.NewEncoder(stdout).Encode(report)
	case "tax-quote", "build-nfe", "build-dps", "build-dps-homolog":
		if *output == "" || *kind != "" {
			return errors.New("--out is required; --kind is not used")
		}
	case "validate":
		if *kind == "" || *output != "" {
			return errors.New("validate requires --kind and does not write an output")
		}
	default:
		return errors.New("unsupported command; no network transmission is available")
	}
	raw, err := readRegular(*input)
	if err != nil {
		return err
	}
	var result []byte
	switch args[0] {
	case "tax-quote":
		var in struct {
			Context fiscalcore.TaxContext `json:"context"`
			Item    fiscalcore.TaxItem    `json:"item"`
			Rules   []fiscalcore.TaxRule  `json:"rules"`
		}
		if err = decode(raw, &in); err != nil {
			return err
		}
		quote, e := fiscalcore.Calculate(in.Context, in.Item, in.Rules)
		if e != nil {
			return e
		}
		result, err = json.MarshalIndent(quote, "", "  ")
	case "build-nfe":
		var in fiscalcore.Invoice
		if err = decode(raw, &in); err != nil {
			return err
		}
		result, _, err = fiscalcore.BuildInvoice(in)
	case "build-dps", "build-dps-homolog":
		var in fiscalcore.DPSInput
		if err = decode(raw, &in); err != nil {
			return err
		}
		if args[0] == "build-dps-homolog" {
			result, _, err = fiscalcore.BuildDPSHomolog(in)
		} else {
			result, _, err = fiscalcore.BuildDPS(in)
		}
	case "validate":
		if err = fiscalcore.ValidateXML(*kind, raw); err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, "XSD valid; this does not verify signature, authorization or homologation.")
		return err
	}
	if err != nil {
		return err
	}
	if err = writeNew(*output, result); err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, "Created offline artifact; no official fiscal document was issued.")
	return err
}
