package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/nathan-osman/go-sunrise"
)

var page = `
<!DOCTYPE html>
<html lang="en" class="no-js">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta http-equiv="refresh" content="2" >


  <title>poules</title>

  <!--
  <meta name="description" content="Page description">
  <link rel="canonical" href="https://">
  <link rel="icon" href="/favicon.ico">
  <link rel="icon" href="/favicon.svg" type="image/svg+xml">
  <link rel="apple-touch-icon" href="/apple-touch-icon.png">
  <link rel="manifest" href="/my.webmanifest">
  <meta name="theme-color" content="#FF00FF">
  -->
  <style>
  body {
  	margin: 0;
  	background: beige;
  	color: brown;
  }
  a {
	text-decoration: none;
	color: inherit;
   }

  .domain a {
	float: right;
  }
  .domain {
  	font-size: 24px;
  	padding: 8px;
  	margin: 0;
  	display: block;
	border-bottom: solid brown 1px;
	color: inherit;
  }
  .domain:hover {
  	background: brown;
	color: beige;
  }
  .action:hover {
  	background: brown;
	color: beige;
  }
  .action {
  	font-size: 24px;
  	margin: 0;
  	padding: 8px;
  	margin: 4px;
	color: inherit;
	border: solid brown 1px;
  }
  .poweractions {
  	padding: 16px;
  }
  .uptime {
  	font-size: 24px;
  	padding: 8px;
  	margin: 0;
  	display: block;
	border-bottom: solid brown 1px;
	color: inherit;
  }
  </style>
</head>

<body>
%s
<br>
<img src="/cam/latest.jpg" width="100%">
</body>
</html>
`

type vm struct {
	Uuid    string `json:"uuid"`
	Name    string `json:"name"`
	Started bool   `json:"started"`
}

func main() {

	g := newGPIO()
	defer g.close()
	p := poules{
		gpio: g,
	}

	var nextreconcile, rise, set time.Time
	go func() {
		sleeptime := 30 * time.Second
		now := time.Now()
		rise, set = sunrise.SunriseSunset(46.8, 4.9, now.Year(), now.Month(), now.Day())
		for {
			nextreconcile = time.Now().Add(sleeptime)
			time.Sleep(sleeptime)
			now = time.Now()
			closetime := set.Add(17 * time.Minute) // wait a bit after sunset to close
			if now.After(closetime) {
				// between sunset and midnight: get the sunrise of the next day
				tomorow := now.Add(24 * time.Hour)
				rise, closetime = sunrise.SunriseSunset(46.8, 4.9, tomorow.Year(), tomorow.Month(), tomorow.Day())
				closetime = closetime.Add(17 * time.Minute) // wait a bit after sunset to close
			}

			// keep open/close if explicitly asked in the UI
			// timeout delay?
			if now.After(rise) && now.Before(closetime) {
				// day
				if g.doorState != doorOpen {
					log.Println("opening door after sunrise")
					g.openDoor()
					log.Printf("door is %s will close in %s", g.doorState, closetime.Sub(now))
				}
				sleeptime = closetime.Sub(now).Round(time.Second) + time.Second
			} else {
				// night

				sleeptime = rise.Sub(now).Round(time.Second) + time.Second
				if g.doorState != doorClosed {
					log.Println("closing door after sunset")
					g.closeDoor()
					log.Printf("door is %s, it will open in %s", g.doorState, sleeptime)
				}
			}

			if sleeptime < 0 {
				log.Println("ERROR: negative sleep time", sleeptime)
				sleeptime = time.Minute
			}
			log.Println("going to sleep for:", sleeptime)
		}
	}()

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		var content string

		cmd := exec.Command("uptime", "-p")
		out, err := cmd.CombinedOutput()
		if err != nil {
			content += fmt.Sprintf("failed get uptime: %s", err)
		} else {
			content += fmt.Sprintf(
				"<div class=\"uptime\">%s rise: %s set: %s now: %s</div>",
				out,
				rise.Local().Format(time.Kitchen),
				set.Local().Format(time.Kitchen),
				time.Now().Local().Format(time.Kitchen),
			)
		}

		content += fmt.Sprintf("<div class=\"domain\">%s (will reconcile in %s)<a href=\"#\"></a></div>", p.gpio.doorState, time.Until(nextreconcile).Round(time.Second))

		content += `<div class="poweractions">`
		if p.gpio.doorState != "opening" && p.gpio.doorState != "closing" {
			content += `
	<span class="action"><a href="/open">Open</a></span>
	<span class="action"><a href="/close">Close</a></span>
	`
		}

		content += "</div>"
		fmt.Fprintf(w, page, content)
	})

	r.Get("/open", p.HandleOpen)
	r.Get("/close", p.HandleClose)
	r.Get("/cam/latest.jpg", func(w http.ResponseWriter, r *http.Request) {
		pic, e := os.Open("/home/pi/cam/latest.jpg")
		if e != nil {
			log.Print("failed to open cam ", e)
		}
		io.Copy(w, pic)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
	log.Print(http.ListenAndServe(":"+port, r))
}

type poules struct {
	gpio *gpio
}

func (p poules) HandleOpen(w http.ResponseWriter, r *http.Request) {
	log.Println("recieved door open request")
	log.Println("door opening")
	http.Redirect(w, r, "/", 302)
	go func() {
		p.gpio.openDoor()
		log.Println("door open")
	}()
}
func (p poules) HandleClose(w http.ResponseWriter, r *http.Request) {
	log.Println("recieved door close request")
	log.Println("door closing")
	http.Redirect(w, r, "/", 302)
	go func() {
		p.gpio.closeDoor()
		log.Println("door closed")
	}()
}
