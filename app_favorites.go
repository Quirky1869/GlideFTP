package main

// Favourite folders and files shown pinned at the top of each panel (v1.8.0).

import "GlideFTP/internal/favorites"

func (a *App) GetFavorites() map[string][]favorites.Favorite {
	return a.favMgr.All()
}

func (a *App) SetFavorites(key string, list []favorites.Favorite) error {
	return a.favMgr.Set(key, list)
}
