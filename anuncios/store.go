package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

var ErrNoExiste = errors.New("el anuncio no existe")

const esquema = `
CREATE TABLE IF NOT EXISTS anuncios (
	id     INTEGER PRIMARY KEY AUTOINCREMENT,
	nombre TEXT NOT NULL,
	texto  TEXT NOT NULL DEFAULT '',
	url    TEXT NOT NULL DEFAULT '',
	logo   TEXT NOT NULL DEFAULT '',
	inicio TEXT NOT NULL,
	fin    TEXT NOT NULL,
	activo INTEGER NOT NULL DEFAULT 1,
	notas  TEXT NOT NULL DEFAULT '',
	creado TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS anuncio_pueblos (
	anuncio_id INTEGER NOT NULL REFERENCES anuncios(id) ON DELETE CASCADE,
	pueblo     TEXT NOT NULL,
	PRIMARY KEY (anuncio_id, pueblo)
);
CREATE INDEX IF NOT EXISTS anuncio_pueblos_por_pueblo ON anuncio_pueblos (pueblo);
CREATE TABLE IF NOT EXISTS estadisticas (
	anuncio_id  INTEGER NOT NULL REFERENCES anuncios(id) ON DELETE CASCADE,
	pueblo      TEXT NOT NULL,
	dia         TEXT NOT NULL,
	impresiones INTEGER NOT NULL DEFAULT 0,
	clics       INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (anuncio_id, pueblo, dia)
);`

type Store struct{ db *sql.DB }

type Anuncio struct {
	ID          int64
	Nombre      string
	Texto       string
	URL         string
	Logo        string // nombre del archivo dentro de DATA_DIR/logos
	Inicio      string // AAAA-MM-DD, incluido
	Fin         string // AAAA-MM-DD, incluido
	Activo      bool
	Notas       string
	Pueblos     []string // slugs de la web, p. ej. "alfarras"
	Impresiones int64
	Clics       int64
}

// EstadoEn dice si el anuncio se está mostrando en la fecha dada (AAAA-MM-DD).
func (a Anuncio) EstadoEn(hoy string) string {
	switch {
	case !a.Activo:
		return "Pausado"
	case hoy < a.Inicio:
		return "Programado"
	case hoy > a.Fin:
		return "Caducado"
	default:
		return "Activo"
	}
}

type FilaEstadistica struct {
	Clave       string // pueblo o día
	Impresiones int64
	Clics       int64
}

func abrirStore(ruta string) (*Store, error) {
	dsn := "file:" + ruta + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Una sola conexión: el tráfico es pequeño y así SQLite nunca devuelve "database is locked".
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(esquema); err != nil {
		db.Close()
		return nil, fmt.Errorf("no se pudo preparar la base de datos %s: %w", ruta, err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

const consultaAnuncio = `SELECT a.id, a.nombre, a.texto, a.url, a.logo, a.inicio, a.fin, a.activo, a.notas,
	COALESCE((SELECT group_concat(p.pueblo, ',') FROM anuncio_pueblos p WHERE p.anuncio_id = a.id), ''),
	COALESCE((SELECT SUM(e.impresiones) FROM estadisticas e WHERE e.anuncio_id = a.id), 0),
	COALESCE((SELECT SUM(e.clics) FROM estadisticas e WHERE e.anuncio_id = a.id), 0)
FROM anuncios a`

func escanearAnuncio(fila interface{ Scan(...any) error }) (Anuncio, error) {
	var a Anuncio
	var pueblos string
	err := fila.Scan(&a.ID, &a.Nombre, &a.Texto, &a.URL, &a.Logo, &a.Inicio, &a.Fin, &a.Activo, &a.Notas,
		&pueblos, &a.Impresiones, &a.Clics)
	if pueblos != "" {
		a.Pueblos = strings.Split(pueblos, ",")
		sort.Strings(a.Pueblos)
	}
	return a, err
}

func (s *Store) Listar(ctx context.Context) ([]Anuncio, error) {
	filas, err := s.db.QueryContext(ctx, consultaAnuncio+` ORDER BY a.fin DESC, a.nombre COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var lista []Anuncio
	for filas.Next() {
		a, err := escanearAnuncio(filas)
		if err != nil {
			return nil, err
		}
		lista = append(lista, a)
	}
	return lista, filas.Err()
}

func (s *Store) Obtener(ctx context.Context, id int64) (Anuncio, error) {
	a, err := escanearAnuncio(s.db.QueryRowContext(ctx, consultaAnuncio+` WHERE a.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Anuncio{}, ErrNoExiste
	}
	return a, err
}

// Guardar crea el anuncio (si ID es 0) o lo actualiza, y reemplaza sus pueblos.
func (s *Store) Guardar(ctx context.Context, a *Anuncio) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if a.ID == 0 {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO anuncios (nombre, texto, url, logo, inicio, fin, activo, notas) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			a.Nombre, a.Texto, a.URL, a.Logo, a.Inicio, a.Fin, a.Activo, a.Notas)
		if err != nil {
			return err
		}
		if a.ID, err = res.LastInsertId(); err != nil {
			return err
		}
	} else {
		res, err := tx.ExecContext(ctx,
			`UPDATE anuncios SET nombre = ?, texto = ?, url = ?, logo = ?, inicio = ?, fin = ?, activo = ?, notas = ? WHERE id = ?`,
			a.Nombre, a.Texto, a.URL, a.Logo, a.Inicio, a.Fin, a.Activo, a.Notas, a.ID)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return ErrNoExiste
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM anuncio_pueblos WHERE anuncio_id = ?`, a.ID); err != nil {
			return err
		}
	}
	for _, pueblo := range a.Pueblos {
		if _, err := tx.ExecContext(ctx, `INSERT INTO anuncio_pueblos (anuncio_id, pueblo) VALUES (?, ?)`, a.ID, pueblo); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Borrar elimina el anuncio con sus pueblos y estadísticas, y devuelve su logo para borrar el archivo.
func (s *Store) Borrar(ctx context.Context, id int64) (string, error) {
	var logo string
	err := s.db.QueryRowContext(ctx, `DELETE FROM anuncios WHERE id = ? RETURNING logo`, id).Scan(&logo)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoExiste
	}
	return logo, err
}

// ActivosPorPueblo devuelve los anuncios que se deben mostrar hoy en un pueblo.
func (s *Store) ActivosPorPueblo(ctx context.Context, pueblo, hoy string) ([]Anuncio, error) {
	filas, err := s.db.QueryContext(ctx, `SELECT a.id, a.nombre, a.texto, a.url, a.logo
		FROM anuncios a JOIN anuncio_pueblos p ON p.anuncio_id = a.id
		WHERE p.pueblo = ? AND a.activo = 1 AND a.inicio <= ? AND a.fin >= ?`, pueblo, hoy, hoy)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var lista []Anuncio
	for filas.Next() {
		var a Anuncio
		if err := filas.Scan(&a.ID, &a.Nombre, &a.Texto, &a.URL, &a.Logo); err != nil {
			return nil, err
		}
		lista = append(lista, a)
	}
	return lista, filas.Err()
}

func (s *Store) SumarImpresiones(ctx context.Context, ids []int64, pueblo, dia string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `INSERT INTO estadisticas (anuncio_id, pueblo, dia, impresiones) VALUES (?, ?, ?, 1)
			ON CONFLICT (anuncio_id, pueblo, dia) DO UPDATE SET impresiones = impresiones + 1`, id, pueblo, dia); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DestinoClic devuelve el enlace del anuncio y si el clic cuenta (anuncio en vigor y asociado a ese pueblo).
func (s *Store) DestinoClic(ctx context.Context, id int64, pueblo, hoy string) (string, bool, error) {
	var destino string
	var cuenta bool
	err := s.db.QueryRowContext(ctx, `SELECT a.url,
		(a.activo = 1 AND a.inicio <= ? AND a.fin >= ? AND
		 EXISTS (SELECT 1 FROM anuncio_pueblos p WHERE p.anuncio_id = a.id AND p.pueblo = ?))
		FROM anuncios a WHERE a.id = ?`, hoy, hoy, pueblo, id).Scan(&destino, &cuenta)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, ErrNoExiste
	}
	return destino, cuenta, err
}

func (s *Store) SumarClic(ctx context.Context, id int64, pueblo, dia string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO estadisticas (anuncio_id, pueblo, dia, clics) VALUES (?, ?, ?, 1)
		ON CONFLICT (anuncio_id, pueblo, dia) DO UPDATE SET clics = clics + 1`, id, pueblo, dia)
	return err
}

// Estadisticas devuelve los totales de un anuncio por pueblo y por día desde la fecha dada.
func (s *Store) Estadisticas(ctx context.Context, id int64, desde string) (porPueblo, porDia []FilaEstadistica, err error) {
	if porPueblo, err = s.filasEstadistica(ctx, `SELECT pueblo, SUM(impresiones), SUM(clics) FROM estadisticas
		WHERE anuncio_id = ? GROUP BY pueblo ORDER BY SUM(impresiones) DESC, pueblo`, id); err != nil {
		return nil, nil, err
	}
	porDia, err = s.filasEstadistica(ctx, `SELECT dia, SUM(impresiones), SUM(clics) FROM estadisticas
		WHERE anuncio_id = ? AND dia >= ? GROUP BY dia ORDER BY dia DESC`, id, desde)
	return porPueblo, porDia, err
}

func (s *Store) filasEstadistica(ctx context.Context, consulta string, args ...any) ([]FilaEstadistica, error) {
	filas, err := s.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var lista []FilaEstadistica
	for filas.Next() {
		var f FilaEstadistica
		if err := filas.Scan(&f.Clave, &f.Impresiones, &f.Clics); err != nil {
			return nil, err
		}
		lista = append(lista, f)
	}
	return lista, filas.Err()
}
