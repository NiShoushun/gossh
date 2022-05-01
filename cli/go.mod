module github.com/nishoushun/gossh/cli

go 1.18

require github.com/nishoushun/gossh v0.0.0-00010101000000-000000000000

require (
	golang.org/x/crypto v0.0.0-20220321153916-2c7772ba3064 // indirect
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
    github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
    github.com/stretchr/testify v1.7.1 // indirect
    gopkg.in/alecthomas/kingpin.v2 v2.2.6
)

replace github.com/nishoushun/gossh => ../
