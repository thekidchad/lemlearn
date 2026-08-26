package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/library"
)

// handleListLibrary renvoie la bibliothèque.
//
// L'équipe voit tout, y compris ce qui n'est pas publié ; un organisme ne voit
// que ce qui lui est ouvert.
func handleListLibrary(deps Deps, publishedOnly bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Library == nil {
			writeError(w, http.StatusServiceUnavailable, "bibliothèque indisponible")
			return
		}

		courses, err := deps.Library.ListCourses(r.Context(), publishedOnly)
		if err != nil {
			deps.Log.Error("bibliothèque", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"courses": list(courses)})
	}
}

// handleGetLibraryCourse renvoie une formation de la bibliothèque et ses
// modules.
func handleGetLibraryCourse(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Library == nil {
			writeError(w, http.StatusServiceUnavailable, "bibliothèque indisponible")
			return
		}

		course, modules, err := deps.Library.Course(r.Context(), chi.URLParam(r, "courseID"))
		if err != nil {
			respondNotFound(w, err, "formation introuvable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"course": course, "modules": list(modules)})
	}
}

// handleSaveLibraryCourse crée ou modifie une formation de la bibliothèque.
func handleSaveLibraryCourse(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Library == nil {
			writeError(w, http.StatusServiceUnavailable, "bibliothèque indisponible")
			return
		}

		var course library.Course
		if !decodeJSON(w, r, &course) {
			return
		}
		if id := chi.URLParam(r, "courseID"); id != "" {
			course.ID = id
		}

		saved, err := deps.Library.SaveCourse(r.Context(), course)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

// handleSaveLibraryModule ajoute ou modifie un module de la bibliothèque.
func handleSaveLibraryModule(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Library == nil {
			writeError(w, http.StatusServiceUnavailable, "bibliothèque indisponible")
			return
		}

		var module library.Module
		if !decodeJSON(w, r, &module) {
			return
		}
		module.CourseID = chi.URLParam(r, "courseID")

		saved, err := deps.Library.SaveModule(r.Context(), module)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

// handleDeleteLibraryCourse retire une formation de la bibliothèque.
func handleDeleteLibraryCourse(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Library == nil {
			writeError(w, http.StatusServiceUnavailable, "bibliothèque indisponible")
			return
		}

		if err := deps.Library.DeleteCourse(r.Context(), chi.URLParam(r, "courseID")); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	}
}

// handleImportLibraryCourse recopie une formation dans l'organisation.
func handleImportLibraryCourse(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Library == nil {
			writeError(w, http.StatusServiceUnavailable, "bibliothèque indisponible")
			return
		}

		course, modules, err := deps.Library.Import(r.Context(), session.OrgID, chi.URLParam(r, "courseID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"course":  course,
			"modules": modules,
			// La copie arrive en brouillon : le formateur l'adapte à ses
			// moyens et à son public avant de la publier — c'est lui qui
			// l'assume devant un auditeur.
			"note": "importée en brouillon : relisez les mentions avant de publier",
		})
	}
}
