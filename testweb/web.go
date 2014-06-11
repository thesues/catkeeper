package main

import "github.com/codegangsta/martini"
import "github.com/codegangsta/martini-contrib/render"
import "database/sql"
import _ "github.com/mattn/go-sqlite3"

func main() {
    /* init database */
    db, err := sql.Open("sqlite3", "/tmp/post_db.bin")
    if err != nil {
	    checkErr(err, "open database failed")
	    return
    }
    defer db.Close()


    m := martini.Classic()
    m.Use(render.Renderer())


    m.Get("/", func(r render.Render){
	    pm := getListofPhysicalMachine(db)
	    r.HTML(200, "list" , pm)
    })

    m.Get("/hello/:name", func(params martini.Params, r render.Render) string {
	      return "Hello " + params["name"]
    })


    m.Run()
}
