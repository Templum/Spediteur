package main

import (
	"github.com/Templum/Spediteur/controller"
	"github.com/valyala/fasthttp"
)

func main() {

	handler := controller.NewForwardHandler()
	fasthttp.ListenAndServe(":8888", handler)

}
