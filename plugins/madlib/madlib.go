// © 2016 the CatBase Authors under the WTFPL license. See AUTHORS for the list of authors.

// Package zork implements a zork plugin for catbase.
package madlib

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/velour/catbase/bot"
	"github.com/velour/catbase/bot/msg"
)

// ZorkPlugin is a catbase plugin for playing zork.
type Madlib struct {
	bot bot.Bot
	db  *sqlx.DB
}

func New(b bot.Bot) bot.Handler {
	p := &Madlib{
		bot: b,
		db:  b.DB(),
	}

	// check/create tables

	_, err := p.db.Exec(`create table if not exists madlib (
			id integer primary key,
			name string,
			format string
		);`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = p.db.Exec(`create table if not exists madlib_fields (
			id integer primary key,
			field string,
			value string
		);`)
	if err != nil {
		log.Fatal(err)
	}

	return p
}

func lg(s string, args ...interface{}) {
	log.Printf("[madlib] "+s, args)
}

type field struct {
	Id    sql.NullInt64
	Field string
	Value string
}

type mad struct {
	*sqlx.DB

	Id     sql.NullInt64
	Name   string
	Format string
	Fields []field
}

func (p *Madlib) removeField(field, value string) error {
	_, err := p.db.Exec("DELETE FROM madlib_fields WHERE field=? AND value=?",
		field, value)
	return err
}

func (p *Madlib) addField(field, value string) error {
	_, err := p.db.Exec("INSERT INTO madlib_fields (field, value) VALUES (?, ?)",
		field, value)
	return err
}

func (p *Madlib) newMad(name, format string) (*mad, error) {
	_, err := p.db.Exec("INSERT INTO madlib (name, format) VALUES (?, ?)", name, format)
	if err != nil {
		return nil, err
	}
	return p.getMadlib(name)
}

func (p *Madlib) delMad(name string) error {
	_, err := p.db.Exec("DELETE FROM madlib WHERE name=?", name)
	return err
}

func (p *Madlib) listMad() ([]string, error) {
	var list []string
	err := p.db.Select(&list, "SELECT name FROM madlib")
	return list, err
}

func (p *Madlib) getMadlib(name string) (*mad, error) {
	var libs []mad
	err := p.db.Select(&libs, "SELECT * FROM madlib WHERE name=? LIMIT 1", name)
	if err != nil {
		return nil, err
	}
	if len(libs) == 0 {
		return nil, err
	}
	libs[0].DB = p.db
	return &libs[0], nil
}

func (m *mad) String() string {
	// make myself into a real boy
	re := regexp.MustCompile("{([^}]+)}")
	matches := re.FindAllStringSubmatch(m.Format, -1)
	fs := []string{}
	for _, m := range matches {
		fs = append(fs, m[1])
	}

	var fields []field
	sql := `select * from 
			(select * from madlib_fields where field in (?) order by random(*))
		group by field`

	query, args, err := sqlx.In(sql, fs)
	if err != nil {
		lg("%s", err)
		return fmt.Sprintf("error: %s", err)
	}
	query = m.Rebind(query)
	err = m.Select(&fields, query, args...)
	if err != nil {
		lg("%s", err)
		return fmt.Sprintf("error: %s", err)
	}

	out := m.Format
	for _, f := range fields {
		out = strings.Replace(out, "{"+f.Field+"}", f.Value, -1)
	}
	return out
}

func (p *Madlib) Message(message msg.Message) bool {
	m := strings.ToLower(message.Body)
	ch := message.Channel

	lib, err := p.getMadlib(m)
	if err != nil {
		lg("%s", err)
		p.bot.SendMessage(ch, "There was a problem.")
		return true
	} else if lib != nil && lib.Id.Valid {
		p.bot.SendMessage(ch, lib.String())
		return true
	}

	ms := strings.Fields(strings.TrimPrefix(m, "madlib"))
	cmd := message.Command

	if !cmd || len(ms) == 0 || !strings.HasPrefix(m, "madlib") {
		return false
	}

	lg("ms: %%#v", ms)

	switch {
	case ms[0] == "create" && len(ms) >= 3:
		mad, err := p.newMad(ms[1], strings.Join(ms[2:], " "))
		if err != nil {
			lg("%s", err)
			p.bot.SendMessage(ch, "Something went horribly wrong.")
		} else {
			p.bot.SendMessage(ch, mad.String())
		}
	case ms[0] == "delete" && len(ms) == 2:
		err := p.delMad(ms[1])
		if err != nil {
			lg("%s", err)
			p.bot.SendMessage(ch, "Something went horribly wrong.")
		} else {
			p.bot.SendMessage(ch, "Deleted.")
		}
	case ms[0] == "add" && len(ms) >= 3:
		p.addField(ms[1], strings.Join(ms[2:], " "))
		if err != nil {
			lg("%s", err)
			p.bot.SendMessage(ch, "Something went horribly wrong.")
		} else {
			p.bot.SendMessage(ch, "Added.")
		}
	case ms[0] == "remove" && len(ms) == 3:
		p.removeField(ms[1], strings.Join(ms[2:], " "))
		if err != nil {
			lg("%s", err)
			p.bot.SendMessage(ch, "Something went horribly wrong.")
		} else {
			p.bot.SendMessage(ch, "Removed.")
		}
	case ms[0] == "list" && len(ms) == 1:
		list, err := p.listMad()
		if err != nil {
			lg("%s", err)
			p.bot.SendMessage(ch, "Something went horribly wrong.")
		} else {
			p.bot.SendMessage(ch, strings.Join(list, ", "))
		}
	default:
		p.Help(ch, []string{})
	}

	return true
}

func (p *Madlib) Event(_ string, _ msg.Message) bool { return false }

func (p *Madlib) BotMessage(_ msg.Message) bool { return false }

func (p *Madlib) Help(ch string, _ []string) {
	msg := "Address me and use the command madlib with the following:\n"
	msg += "\t`create <madlib name> <format>` - make a new madlib\n"
	msg += "\t`delete <madlib name>` - remove a madlib\n"
	msg += "\t`add <field> <value>` - add a format field value\n"
	msg += "\t`remove <field> <value>` - remove a format field value\n"
	msg += "\t`list` - list all current madlibs\n"
	msg += "Format is a string with a field represented as `{field}`"
	p.bot.SendMessage(ch, msg)
}

func (p *Madlib) RegisterWeb() *string { return nil }
