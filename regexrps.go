package main

import "regexp"

var (
	rxMethod     = "(\\p{L}+)"
	rxPath       = "((?:/[\\p{L}\\p{N}\\p{Pd}\\p{Pc}{}\\-\\.\\?_~%!$&'()*+,;=:@/]*)+/?)"
	rxTags       = "(\\p{L}[\\p{L}\\p{N}\\p{Pd}\\.\\p{Pc}\\p{Zs}]+)"
	rxID         = "((?:\\p{L}[\\p{L}\\p{N}\\p{Pd}\\p{Pc}]+)+)"
	rxStatusCode = "(\\p{N}+)"

	rxSwag  = regexp.MustCompile(`swag:([\p{L}\p{N}\p{Pd}\p{Pc}]+)`)
	rxRoute = regexp.MustCompile(
		"swag:route\\p{Zs}*" +
			rxID + "\\p{Zs}*" +
			rxMethod + "\\p{Zs}*" +
			rxPath + "(?:\\p{Zs}+" +
			rxTags + ")?\\p{Zs}*$")
	rxReq = regexp.MustCompile(
		"swag:req\\p{Zs}*" +
			rxID + "\\p{Zs}*$")
	rxAns = regexp.MustCompile(
		"swag:ans\\p{Zs}*" +
			rxID + "\\p{Zs}*" +
			rxStatusCode + "\\p{Zs}*$")

	rxSpace         = regexp.MustCompile(`\p{Zs}+`)
	rxStripComments = regexp.MustCompile(`^[^\p{L}\p{N}\p{Pd}\p{Pc}\+]*`)
)
