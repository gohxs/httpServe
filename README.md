httpServe
=============

Simple tool I wrote and use to serve current directory contents without
any hustle, it renders markdown using clientside [strapdownjs](http://strapdownjs.com/)
and does live reload when files changed

Usage:

```bash
go get github.com/gohxs/httpServe
# with $GOPATH/bin in  $PATH
httpServe
```

Changelog
---------

* Added support for live reload of certain files
  * markdown
