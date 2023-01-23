package magnetware

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo"
	"github.com/xgfone/bt/bencode"
	"github.com/xgfone/bt/metainfo"
	"github.com/xgfone/bttools/commands/torrent"
)

type Magnet struct {
	*os.File
	Link string
}

type MagnetWare struct {
	cache   map[string]*Magnet
	BaseDir string
}

func NewMagnetWare(bd string) *MagnetWare {
	return &MagnetWare{
		BaseDir: bd,
		cache:   make(map[string]*Magnet),
	}

}

func (m *MagnetWare) Magnet(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(m.BaseDir, r.URL.Path)
		if f, err := os.Open(path); err == nil {
			if s, err := f.Stat(); err == nil {
				cachedFile, ok := m.cache[path]
				if ok {
					if t, err := cachedFile.Stat(); err == nil {
						if s.ModTime().After(t.ModTime()) {
							var config torrent.CreateTorrentConfig
							config.RootDir = path
							config.Announces = append(config.Announces, "http://w7tpbzncbcocrqtwwm3nezhnnsw4ozadvi2hmvzdhrqzfxfum7wa.b32.i2p/a")
							config.Name = path
							//config.Announces
							config.WebSeeds = append(config.WebSeeds, r.URL.String())
							if mi, err := CreateTorrent(config); err == nil {
								mag := mi.Magnet(config.Name, mi.InfoHash())
								m.cache[path].Link = mag.String()
								w.Header().Add("x-i2p-magnet", m.cache[path].Link)
							}
						} else {
							w.Header().Add("x-i2p-magnet", cachedFile.Link)
						}
					}
				} else {
					m.cache[path] = &Magnet{
						File: f,
					}
					var config torrent.CreateTorrentConfig
					config.RootDir = path
					config.Announces = append(config.Announces, "http://w7tpbzncbcocrqtwwm3nezhnnsw4ozadvi2hmvzdhrqzfxfum7wa.b32.i2p/a")
					config.Name = path
					//config.Announces
					config.WebSeeds = append(config.WebSeeds, r.URL.String())
					if mi, err := CreateTorrent(config); err == nil {
						mag := mi.Magnet(config.Name, mi.InfoHash())
						w.Header().Add("x-i2p-magnet", mag.String())
						m.cache[path].Link = mag.String()
					}
				}
			}
		}
		fmt.Println("Executing middleware before request phase!")
		// Pass control back to the handler
		handler.ServeHTTP(w, r)
	})
}

func (m *MagnetWare) EchoMagnet() echo.MiddlewareFunc {
	return echo.WrapMiddleware(m.Magnet)
}

// CreateTorrent creates a .torrent file.
func CreateTorrent(config torrent.CreateTorrentConfig) (*metainfo.MetaInfo, error) {
	info, err := metainfo.NewInfoFromFilePath(config.RootDir, config.PieceLength)
	if err != nil {
		return nil, err
	}

	if config.Name != "" {
		info.Name = config.Name
	}

	var mi metainfo.MetaInfo
	mi.Comment = config.Comment
	mi.InfoBytes, err = bencode.EncodeBytes(info)
	if err != nil {
		return nil, err
	}

	switch len(config.Announces) {
	case 0:
	case 1:
		mi.Announce = config.Announces[0]
	default:
		mi.AnnounceList = metainfo.AnnounceList{config.Announces}
	}

	for _, seed := range config.WebSeeds {
		mi.URLList = append(mi.URLList, seed)
	}

	if !config.NoDate {
		mi.CreationDate = time.Now().Unix()
	}

	var out io.WriteCloser = os.Stdout
	if config.Output != "" {
		out, err = os.OpenFile(config.Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return nil, err
		}
		defer out.Close()
	}

	return &mi, nil
}
