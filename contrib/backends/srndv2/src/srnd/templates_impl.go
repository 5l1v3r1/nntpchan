//
// templates.go
// template model interfaces
//
package srnd

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type templateEngine struct {
	// loaded templates
	templates map[string]string
	// root directory for templates
	template_dir string
	// mutex for accessing templates
	templates_mtx sync.RWMutex
	// do we want to minimize the html generated?
	Minimize bool
	// database
	DB Database
	// template driver
	driver TemplateDriver
}

func (self *templateEngine) templateCached(name string) (ok bool) {
	self.templates_mtx.Lock()
	_, ok = self.templates[name]
	self.templates_mtx.Unlock()
	return
}

// explicitly reload a template
func (self *templateEngine) reloadTemplate(name string) {
	self.templates_mtx.Lock()
	self.templates[name] = self.loadTemplate(name)
	self.templates_mtx.Unlock()
}

// check if we have this template
func (self *templateEngine) hasTemplate(name string) bool {
	return CheckFile(self.templateFilepath(name))
}

// explicitly reload all loaded templates
func (self *templateEngine) reloadAllTemplates() {
	loadThese := []string{}
	// get all the names of the templates we have loaded
	self.templates_mtx.Lock()
	for tname, _ := range self.templates {
		loadThese = append(loadThese, tname)
	}
	self.templates_mtx.Unlock()
	// for each template we have loaded, reload the contents from file
	for _, tname := range loadThese {
		self.reloadTemplate(tname)
	}
}

// get cached post model from cache after updating it
func (self *templateEngine) updatePostModel(prefix, frontend, msgid, rootmsgid, group string, db Database) PostModel {
	return db.GetPostModel(prefix, msgid)
	/*
		// get board
		self.groups_mtx.Lock()
		board := self.groups[group]
		self.groups_mtx.Unlock()

		var th ThreadModel
		if msgid == rootmsgid {
			// new thread
			if len(board) > 0 {
				page := board[0]
				page.Update(db)
				th = page.GetThread(rootmsgid)
			}
		} else {
			// reply
			for _, page := range board {
				t := page.GetThread(rootmsgid)
				if t != nil {
					th = t
					th.Update(db)
					break
				}
			}
		}
		if th == nil {
			// reload board, this will be a heavy operation
			board.UpdateAll(db)
			// find it
			for _, page := range board {
				t := page.GetThread(rootmsgid)
				if t != nil {
					th = t
					th.Update(db)
					break
				}
			}
			for _, page := range board {
				updateLinkCacheForBoard(page)
			}
			self.groups_mtx.Lock()
			self.groups[group] = board
			self.groups_mtx.Unlock()
		}
		if th == nil {
			if rootmsgid == msgid {
				return db.GetPostModel(prefix, rootmsgid)
			}
			log.Println("template could not find thread", rootmsgid, "in", group)
			return nil
		}

		// found
		m := th.OP()
		if m.MessageID() == msgid {
			return m
		}
		for _, p := range th.Replies() {
			if p.MessageID() == msgid {
				// found as reply
				return p
			}
		}
		log.Println("template could not find post model for thread", rootmsgid, "in", group)
		// not found
		return nil
	*/
}

// get the filepath to a template
func (self *templateEngine) templateFilepath(name string) string {
	if strings.Count(name, "..") > 0 {
		return ""
	}
	return filepath.Join(self.template_dir, name+self.driver.Ext())
}

// load a template from file, return as string
func (self *templateEngine) loadTemplate(name string) (t string) {
	b, err := ioutil.ReadFile(self.templateFilepath(name))
	if err == nil {
		t = string(b)
	} else {
		log.Println("error loading template", err)
		t = err.Error()
	}
	return
}

// get a template, if it's not cached load from file and cache it
func (self *templateEngine) getTemplate(name string) (t string) {
	if !self.templateCached(name) {
		self.templates_mtx.Lock()
		self.templates[name] = self.loadTemplate(name)
		self.templates_mtx.Unlock()
	}
	self.templates_mtx.Lock()
	t, _ = self.templates[name]
	self.templates_mtx.Unlock()
	return
}

// render a template, self explanitory
func (self *templateEngine) renderTemplate(name string, obj map[string]interface{}, i18n *I18N) string {
	t := self.getTemplate(name)
	if i18n == nil {
		i18n = I18nProvider
	}
	obj["i18n"] = i18n
	s, err := self.driver.RenderString(t, obj)
	if err == nil {
		return s
	} else {
		return err.Error()
	}
}

// write a template to an io.Writer
func (self *templateEngine) writeTemplate(name string, obj map[string]interface{}, wr io.Writer, i18n *I18N) (err error) {
	t := self.getTemplate(name)
	if i18n == nil {
		i18n = I18nProvider
	}
	obj["i18n"] = i18n
	return self.driver.Render(t, obj, wr)
}

// easy wrapper for json.NewEncoder
func (self *templateEngine) renderJSON(wr io.Writer, obj interface{}) {
	err := json.NewEncoder(wr).Encode(obj)
	if err != nil {
		log.Println("error rendering json", err)
	}
}

// get a board model given a newsgroup
// load un updated board model if we don't have it
func (self *templateEngine) obtainBoard(prefix, frontend, group string, db Database) (model GroupModel) {
	// warning, we attempt to do smart reloading
	// dark magic may lurk here
	p := db.GetGroupPageCount(group)
	pages := int(p)
	perpage, _ := db.GetThreadsPerPage(group)
	// reload all the pages
	var newModel GroupModel
	for page := 0; page < pages; page++ {
		newModel = append(newModel, db.GetGroupForPage(prefix, frontend, group, page, int(perpage)))
	}
	model = newModel

	return
}

func (self *templateEngine) genCatalog(prefix, frontend, group string, wr io.Writer, db Database, i18n *I18N, sfw bool) {
	board := self.obtainBoard(prefix, frontend, group, db)
	catalog := new(catalogModel)
	catalog.prefix = prefix
	catalog.frontend = frontend
	catalog.board = group
	catalog.I18N(i18n)
	catalog.MarkSFW(sfw)
	for page, bm := range board {
		for _, th := range bm.Threads() {
			th.Update(db)
			catalog.threads = append(catalog.threads, &catalogItemModel{op: th.OP(), page: page, replycount: len(th.Replies())})
		}
	}
	self.writeTemplate("catalog", map[string]interface{}{"board": catalog, "sfw": sfw}, wr, i18n)
}

// generate a board page
func (self *templateEngine) genBoardPage(allowFiles, requireCaptcha bool, prefix, frontend, newsgroup string, pages, page int, wr io.Writer, db Database, json bool, i18n *I18N, invertPagination, sfw bool) {
	// get the board page model
	perpage, _ := db.GetThreadsPerPage(newsgroup)
	var boardPage BoardModel
	if invertPagination {
		boardPage = db.GetGroupForPage(prefix, frontend, newsgroup, int(pages-1)-page, int(perpage))
	} else {
		boardPage = db.GetGroupForPage(prefix, frontend, newsgroup, page, int(perpage))
	}
	boardPage.Update(db)
	boardPage.I18N(i18n)
	boardPage.MarkSFW(sfw)
	// render it
	if json {
		self.renderJSON(wr, boardPage)
	} else {
		form := renderPostForm(prefix, newsgroup, "", allowFiles, requireCaptcha, i18n)
		self.writeTemplate("board", map[string]interface{}{"board": boardPage, "page": page, "form": form, "sfw": sfw}, wr, i18n)
	}
}

func (self *templateEngine) genUkko(prefix, frontend string, wr io.Writer, database Database, json bool, i18n *I18N, invertPagination, sfw bool) {
	var page int64
	pages, err := database.GetUkkoPageCount(10)
	if invertPagination {
		page = pages
	}
	if err == nil {
		self.genUkkoPaginated(prefix, frontend, wr, database, int(pages), int(page), json, i18n, invertPagination, sfw)
	} else {
		log.Println("genUkko()", err.Error())
	}
}

func (self *templateEngine) genUkkoPaginated(prefix, frontend string, wr io.Writer, database Database, pages, page int, json bool, i18n *I18N, invertPagination, sfw bool) {
	var threads []ThreadModel
	var articles []ArticleEntry
	if invertPagination {
		articles = database.GetLastBumpedThreadsPaginated("", 10, (pages-page)*10)
	} else {
		articles = database.GetLastBumpedThreadsPaginated("", 10, page*10)
	}
	for _, article := range articles {
		root := article[0]
		thread, err := database.GetThreadModel(prefix, root)
		if err == nil {
			thread.I18N(i18n)
			thread.MarkSFW(sfw)
			threads = append(threads, thread)
		}
	}
	obj := map[string]interface{}{"prefix": prefix, "threads": threads, "page": page, "sfw": sfw}
	if page > 0 {
		obj["prev"] = map[string]interface{}{"no": page - 1}
	}
	if page < pages {
		obj["next"] = map[string]interface{}{"no": page + 1}
	}
	if json {
		self.renderJSON(wr, obj)
	} else {
		// render ukko navbar
		navbar := make(map[string]interface{})
		navbar["name"] = "Overboard"
		navbar["frontend"] = frontend
		navbar["prefix"] = prefix
		// inject navbar
		obj["navbar"] = self.renderTemplate("navbar", navbar, i18n)
		// render
		self.writeTemplate("ukko", obj, wr, i18n)
	}
}

func (self *templateEngine) genThread(allowFiles, requireCaptcha bool, root ArticleEntry, prefix, frontend string, wr io.Writer, db Database, json bool, i18n *I18N, sfw bool) {
	newsgroup := root.Newsgroup()
	msgid := root.MessageID()

	/*
		if !db.HasArticleLocal(msgid) {
			log.Println("don't have", msgid, "locally, not regenerating")
			return
		}
	*/
	t, err := db.GetThreadModel(prefix, msgid)
	if err == nil {
		t.MarkSFW(sfw)
		if json {
			self.renderJSON(wr, t)
		} else {
			t.I18N(i18n)
			form := renderPostForm(prefix, newsgroup, msgid, allowFiles, requireCaptcha, i18n)
			self.writeTemplate("thread", map[string]interface{}{"sfw": sfw, "thread": t, "board": map[string]interface{}{"Name": newsgroup, "Frontend": frontend, "AllowFiles": allowFiles}, "form": form, "prefix": prefix}, wr, i18n)
		}
	} else {
		log.Println("templates: error getting thread for ", msgid, err.Error())
	}
	/*
		// get the board model, don't update the board
		board := self.obtainBoard(prefix, frontend, newsgroup, false, db)
		// find the thread model in question
		for _, pagemodel := range board {
			t := pagemodel.GetThread(msgid)
			if t != nil {
				// update thread
				t.Update(db)
				// render it
				if json {
					self.renderJSON(wr, t)
				} else {
					form := renderPostForm(prefix, newsgroup, msgid, allowFiles)
					self.writeTemplate("thread.mustache", map[string]interface{}{"thread": t, "board": pagemodel, "form": form}, wr)
				}
				return
			}
		}
		log.Println("thread not found for message id", msgid)
		return

			// we didn't find it D:
			// reload everything
			// TODO: should we reload everything!?
			b := self.obtainBoard(prefix, frontend, newsgroup, true, db)
			// find the thread model in question
			for _, pagemodel := range b {
				t := pagemodel.GetThread(msgid)
				if t != nil {
					// we found it
					// render thread
					t.Update(db)
					if json {
						self.renderJSON(wr, t)
					} else {
						form := renderPostForm(prefix, newsgroup, msgid, allowFiles)
						self.writeTemplate("thread.mustache", map[string]interface{}{"thread": t, "board": pagemodel, "form": form}, wr)
					}
					self.groups_mtx.Lock()
					self.groups[newsgroup] = b
					self.groups_mtx.Unlock()
					return
				}
			}
			// it's not there wtf
			log.Println("thread not found for message id", msgid)
	*/
}

// change the directory we are using for templates
func (self *templateEngine) changeTemplateDir(dirname string) {
	log.Println("change template directory to", dirname)
	self.template_dir = dirname
	self.reloadAllTemplates()
}

func (self *templateEngine) createNotFoundHandler(prefix, frontend string) (h http.Handler) {
	h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		self.renderNotFound(w, r, prefix, frontend, nil)
	})
	return
}

// default renderer of 404 pages
func (self *templateEngine) renderNotFound(wr http.ResponseWriter, r *http.Request, prefix, frontend string, i18n *I18N) {
	wr.WriteHeader(404)
	opts := make(map[string]interface{})
	opts["prefix"] = prefix
	opts["frontend"] = frontend
	self.writeTemplate("404", opts, wr, i18n)
}

func (self *templateEngine) findLink(prefix, hash string) (url string) {
	ents, _ := self.DB.GetCitesByPostHashLike(hash)
	if len(ents) > 0 {
		url = fmt.Sprintf("%st/%s/#%s", prefix, HashMessageID(ents[0].Reference()), HashMessageID(ents[0].MessageID()))
	}
	return
}

func renderPostForm(prefix, board, op_msg_id string, files, captcha bool, i18n *I18N) string {
	url := prefix + "post/" + board
	button := "New Thread"
	if op_msg_id != "" {
		button = "Reply"
		if i18n != nil {
			b := i18n.Translate("postbutton_reply")
			if b != "" {
				button = b
			}
		}
	} else if i18n != nil {
		b := i18n.Translate("postbutton_thread")
		if b != "" {
			button = b
		}
	}
	return template.renderTemplate("postform", map[string]interface{}{"post_url": url, "reference": op_msg_id, "button": button, "files": files, "prefix": prefix, "DisableCaptcha": !captcha}, i18n)
}

// generate misc graphs
func (self *templateEngine) genGraphs(prefix string, wr io.Writer, db Database, i18n *I18N) {

	//
	// begin gen history.html
	//

	var all_posts postsGraph
	// this may take a bit
	log.Println("getting monthly post history...")
	posts := db.GetMonthlyPostHistory()

	if posts == nil {
		// wtf?
		log.Println("no monthly posts gotten wtfug yo?")
	} else {
		for _, entry := range posts {
			all_posts = append(all_posts, postsGraphRow{
				day: entry.Time(),
				Num: entry.Count(),
			})
		}
	}
	sort.Sort(all_posts)

	_, err := io.WriteString(wr, self.renderTemplate("graph_history", map[string]interface{}{"history": all_posts}, i18n))
	if err != nil {
		log.Println("error writing history graph", err)
	}

	//
	// end gen history.html
	//

}

func (self *templateEngine) genBoardList(prefix, name string, wr io.Writer, db Database, i18n *I18N) {
	// the graph for the front page
	var graph boardPageRows

	stats, err := db.GetNewsgroupStats()
	if err != nil {
		log.Println("error getting board list", err)
		io.WriteString(wr, err.Error())
		return
	}

	for idx := range stats {
		graph = append(graph, boardPageRow{
			Board: stats[idx].Name,
			Day: stats[idx].PPD,
		})
	}
	
	param := map[string]interface{}{
		"prefix":   prefix,
		"frontend": name,
	}
	sort.Sort(graph)
	param["graph"] = graph
	_, err = io.WriteString(wr, self.renderTemplate("boardlist", param, i18n))
	if err != nil {
		log.Println("error writing board list page", err)
	}
}

// generate front page
func (self *templateEngine) genFrontPage(top_count int, prefix, frontend_name string, indexwr, boardswr io.Writer, db Database, i18n *I18N) {

	models := db.GetLastPostedPostModels(prefix, 20)

	for idx := range models {
		models[idx].I18N(i18n)
	}

	wr := indexwr

	param := make(map[string]interface{})

	param["overview"] = self.renderTemplate("overview", map[string]interface{}{"overview": overviewModel(models)}, i18n)
	/*
		sort.Sort(posts_graph)
		param["postsgraph"] = self.renderTemplate("posts_graph.mustache", map[string]interface{}{"graph": posts_graph})

		if len(frontpage_graph) > top_count {
			param["boardgraph"] = frontpage_graph[:top_count]
		} else {
			param["boardgraph"] = frontpage_graph
		}
	*/
	param["frontend"] = frontend_name
	param["totalposts"] = db.ArticleCount()
	param["prefix"] = prefix
	// render and inject navbar
	param["navbar"] = self.renderTemplate("navbar", map[string]interface{}{"name": "Front Page", "frontend": frontend_name, "prefix": prefix}, i18n)

	_, err := io.WriteString(wr, self.renderTemplate("frontpage", param, i18n))
	if err != nil {
		log.Println("error writing front page", err)
	}
	/*
		wr = boardswr

	*/
}
