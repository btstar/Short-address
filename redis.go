package main

import (
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/mattheath/base62"
	"github.com/speps/go-hashids"
	"time"
)

const (
	UrlIdKey           = "next.url.id"         //自增ID，唯一
	ShortLinkKey       = "shortLink:%s:url"    //短域名->hash信息
	UrlHashKey         = "urlHash:%s:url"      //hash->短域名
	ShortLinkDetailKey = "shortLink:%s:detail" //短域名->域名详细信息
)

type RedisCli struct {
	Cli *redis.Pool
	Db  int //数据库
}

//url生成短域名
func (r *RedisCli) Shorten(url string, exp int64) (string, error) {
	urlHash := toHash(url)
	rd := r.Cli.Get()
	defer rd.Close()
	if _, err := rd.Do("select", r.Db); err != nil {
		return "", err
	}
	d, err := redis.String(rd.Do("get", fmt.Sprintf(UrlHashKey, urlHash)))
	if err != nil && err != redis.ErrNil {
		return "", err
	} else {
		if d == "" {
			//空数据
		} else {
			return d, nil
		}
	}
	if _, err := rd.Do("incr", UrlIdKey); err != nil {
		return "", err
	}
	id, err := redis.Int64(rd.Do("get", UrlIdKey))
	if err != nil {
		return "", err
	}
	shortLink := base62.EncodeInt64(id)
	exp *= 60
	if _, err := rd.Do("setex", fmt.Sprintf(ShortLinkKey, shortLink), exp, url); err != nil {
		return "", err
	}

	if _, err := rd.Do("setex", fmt.Sprintf(UrlHashKey, urlHash), exp, shortLink); err != nil {
		return "", nil
	}
	detail, err := json.Marshal(&UrlDetail{
		Url:                 url,
		CreateAt:            time.Now().String(),
		ExpirationInMinutes: time.Duration(exp),
	})
	if err != nil {
		return "", err
	}
	if _, err := rd.Do("setex", fmt.Sprintf(ShortLinkDetailKey, shortLink), exp, detail); err != nil {
		return "", err
	}
	return shortLink, nil
}

//url转换hash
func toHash(url string) string {
	hd := hashids.NewData()
	hd.Salt = url
	hd.MinLength = 0
	h, _ := hashids.NewWithData(hd)
	r, _ := h.Encode([]int{45, 434, 1313, 99})
	return r
}

//定义url详细信息
type UrlDetail struct {
	Url                 string        `json:"url"`
	CreateAt            string        `json:"create_at"`
	ExpirationInMinutes time.Duration `json:"expiration_in_minutes"`
}

//短链接获取域名信息
func (r *RedisCli) ShortLinkInfo(shortLink string) (interface{}, error) {
	rd := r.Cli.Get()
	defer rd.Close()
	if _, err := rd.Do("select", r.Db); err != nil {
		return "", err
	}
	detail, err := rd.Do("get", fmt.Sprintf(ShortLinkDetailKey, shortLink))
	if err != nil {
		return "", err
	} else {
		return detail, nil
	}
}

//短链接获取原始链接
func (r *RedisCli) UnShorten(shortLink string) (string, error) {
	rd := r.Cli.Get()
	defer rd.Close()
	if _, err := rd.Do("select", r.Db); err != nil {
		return "", err
	}
	url, err := redis.String(rd.Do("get", fmt.Sprintf(ShortLinkKey, shortLink)))
	if err != nil {
		return "", err
	} else {
		return url, nil
	}
}

//实力化redis
func NewRedisCli(addr string, pwd string, maxIdle, MaxActive, db int) *RedisCli {
	client := &redis.Pool{
		MaxIdle:     maxIdle,
		MaxActive:   MaxActive,
		IdleTimeout: 240 * time.Second,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", addr, redis.DialPassword(pwd))
			if err != nil {
				return nil, err
			}
			return c, nil
		},
	}
	return &RedisCli{Cli: client, Db: db}
}
