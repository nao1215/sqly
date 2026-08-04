package shell

import (
	"fmt"
	"strings"
)

// Downloading is a thing sqly does on behalf of whoever ran it, and a URL on the
// command line is the one input that reaches out to a machine nobody local
// chose. So it is a capability the session is granted, not a default it has.
//
// The capability lives on the Shell, seeded once from --allow-remote, and every
// path that could produce an HTTP request asks the same function about it:
// positional arguments at startup, `.import` at the prompt, `.import` inside a
// piped script, and `.import` inside a --script-file. There is no package-level
// flag and no environment variable, because a capability that can be granted
// from somewhere other than the invocation is not one the invocation controls.
//
// What this is not:
//
//   - It is not a sandbox. A caller that can add flags can add this one.
//   - It is not an SSRF defense. With the flag given, sqly fetches the URL it
//     was handed, whether that names a public host, localhost, a private range,
//     or a cloud metadata endpoint, and it does not resolve names twice to see
//     whether the answer moved.
//   - It is not a proxy or DNS policy.
//
// What it is: the switch that decides whether sqly makes an HTTP request at all.
// A wrapper, a CI job, or an agent harness that fixes sqly's argument list and
// leaves the flag out has turned sqly's own downloading off, and can then say so
// about the tool it runs.

// errRemoteNotAllowed is the refusal text, kept in one place because the startup
// path, the script path, and the interactive path all say it.
const remoteCapabilityFlag = "--allow-remote"

// remoteCapabilityError builds the refusal for a URL this session may not
// download. It is an invocationError, so it exits 2 as a usage error: the fix is
// on the command line, and nothing was read or written on the way to it.
func remoteCapabilityError(urls []string) error {
	subject := "a remote input"
	if len(urls) == 1 {
		subject = urls[0]
	} else if len(urls) > 1 {
		subject = strings.Join(urls, ", ")
	}
	return &invocationError{Err: fmt.Errorf(
		"sqly will not download %s: reading an http(s) URL needs %s, which this session was not given. Re-run with %s to allow it. %s is a network capability, not a sandbox or an SSRF defense: it decides whether sqly makes a request, not where the request may go",
		subject, remoteCapabilityFlag, remoteCapabilityFlag, remoteCapabilityFlag)}
}

// authorizeRemoteInputs is the one place the policy is decided. It reports the
// first violation across the whole list rather than the first input, so a mixed
// import refuses before anything local has been resolved: `sqly local.csv
// https://host/remote.csv` must not leave the session holding local.csv.
//
// Only http and https reach here as remote. Every other URL-like input (ftp,
// file, ssh, gopher) is refused further down as an unfetchable scheme, with or
// without the capability, and a Windows drive path or a local file name holding
// a colon is not a URL at all — see unfetchableURLScheme.
func (s *Shell) authorizeRemoteInputs(inputs []string) error {
	if s.allowRemote {
		return nil
	}
	var denied []string
	for _, input := range inputs {
		if isRemoteURL(input) {
			denied = append(denied, input)
		}
	}
	if len(denied) == 0 {
		return nil
	}
	return remoteCapabilityError(denied)
}

// authorizeScriptRemoteInputs applies the same policy to the `.import` lines of
// a parsed script, before the script's first statement runs.
//
// Checking the whole script up front rather than at the failing line is what
// makes the refusal clean: no earlier statement has executed, no earlier import
// has happened, and stdout carries nothing. A script that runs half way and then
// discovers it may not download would leave the caller to work out which half.
func (s *Shell) authorizeScriptRemoteInputs(elements []scriptElement) error {
	if s.allowRemote {
		return nil
	}
	for _, element := range elements {
		if element.commandName() != importCommand {
			continue
		}
		// A malformed .import line is the import command's own error to report;
		// here an unparseable line simply has no URL in it.
		argv, err := splitArgs(element.text)
		if err != nil {
			continue
		}
		if err := s.authorizeRemoteInputs(argv[1:]); err != nil {
			return &scriptError{Err: fmt.Errorf("line %d: %w", element.startLine, err)}
		}
	}
	return nil
}
