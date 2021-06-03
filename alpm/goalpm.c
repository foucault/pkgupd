#include <alpm.h>
#include <stdlib.h>
#include <string.h>
#include <stdarg.h>
#include <assert.h>
#include "goalpm.h"

typedef struct _paths_t {
    char *root;
    char *lib;
} paths_t;

paths_t *paths;

paths_t *paths_new(char *_root, char *_lib) {
    paths_t *p = malloc(sizeof(paths_t));
    assert(p != NULL);
    if(_root != NULL) {
        p->root = _root;
    } else {
        p->root = "/";
    }

    if(_lib != NULL) {
        p->lib = _lib;
    } else {
        p->lib = "/var/lib/pacman";
    }

    return p;

}

void free_paths(paths_t *p) {
    free(p->root);
    free(p->lib);
    free(p);
}

void logeverything(alpm_loglevel_t level, const char *fmt, va_list args){
	if(fmt[0] == '\0') {
		return;
	}
	switch(level) {
		case ALPM_LOG_ERROR: printf("error: "); break;
		case ALPM_LOG_WARNING: printf("warning: "); break;
		case ALPM_LOG_DEBUG: printf("debug: "); break;
		default: return; /* skip other messages */
	}
	vprintf(fmt, args);
}

static char* _strdup(const char *str) {
	return str ? strcpy(malloc(strlen(str)+1), str) : NULL;
}

void init_paths(char* root, char* lib){
    if(!paths) {
        paths = paths_new(root, lib);
    }
}

void goalpm_cleanup() {
    free_paths(paths);
    paths = NULL;
}

syncdb* new_syncdb(char* name){
	syncdb* db = (syncdb*)malloc(sizeof(syncdb));
	if( !db ) {
		return NULL;
	}
	db->name = name;
	db->servers = NULL;
	return db;
}

static void free_syncdb(void* db) {
	syncdb* dbb = (syncdb*)db;
	free(dbb->name);
	alpm_list_free(dbb->servers);
	free(dbb);
	dbb = NULL;
}

void free_syncdb_list(alpm_list_t* list) {
	alpm_list_free_inner(list, free_syncdb);
	alpm_list_free(list);
}

static void free_pkg(void* pkg) {
	upd_package* pkgg = (upd_package*)pkg;
	free(pkgg->name);
	free(pkgg->rem_version);
	free(pkgg->loc_version);
	free(pkgg);
	pkgg = NULL;
}

void free_pkg_list(alpm_list_t* list) {
	alpm_list_free_inner(list, free_pkg);
	alpm_list_free(list);
}

void add_server_to_syncdb(syncdb* db, char* server){
	db->servers = alpm_list_add(db->servers, server);
}

static void dump_servers(syncdb* db) {
	alpm_list_t* it;
	for(it = db->servers; it; it=alpm_list_next(it)){
		printf("  %s\n", (char*)it->data);
	}
}

void dump_alpm_servers(alpm_db_t* db) {
	alpm_list_t* it;
	printf("Dumping servers for db %s\n", alpm_db_get_name(db));
	for(it = alpm_db_get_servers(db); it; it=alpm_list_next(it)){
		printf("\t'%s'\n", (const char*)it->data);
	}
}

void dump_syncdb_list(alpm_list_t* list){
	alpm_list_t* it;
	for(it = list; it; it = alpm_list_next(it)){
		syncdb* db = (syncdb*)it->data;
		printf("Found db: \"%s\"\n", db->name);
		dump_servers(db);
	}
}

static alpm_handle_t* create_handle() {
    assert(paths != NULL);
	alpm_errno_t err;
	alpm_handle_t *handle = alpm_initialize(paths->root, paths->lib, &err);
	/*alpm_option_set_logcb(handle, logeverything);*/
	return handle;
}

static void register_sync_dbs(alpm_handle_t* handle, alpm_list_t* syncdbs){
	const alpm_siglevel_t level = ALPM_SIG_DATABASE | ALPM_SIG_DATABASE_OPTIONAL;
	alpm_list_t *it = NULL;
	alpm_list_t *it2 = NULL;
	for(it = syncdbs; it; it=alpm_list_next(it)){
		syncdb *foo = it->data;
		alpm_db_t *db = alpm_register_syncdb(handle, foo->name, level);
		for(it2 = foo->servers; it2; it2=alpm_list_next(it2)) {
			alpm_db_add_server(db, it2->data);
		}
	}
}

static int is_foreign(alpm_handle_t* handle, alpm_pkg_t* pkg) {
	const char *pkgname = alpm_pkg_get_name(pkg);
	alpm_list_t *i;
	alpm_list_t *sync_dbs = alpm_get_syncdbs(handle);
	for(i = sync_dbs; i; i = alpm_list_next(i)) {
		if(alpm_db_get_pkg(i->data, pkgname)) {
			return 0;
		}
	}
	return 1;
}

alpm_list_t* get_updates(alpm_list_t* syncdbs){
	alpm_list_t *it = NULL;
	alpm_list_t *ret = NULL;
	alpm_pkg_t *pkg = NULL;
	alpm_pkg_t *spkg = NULL;

	alpm_handle_t* handle = create_handle();
	alpm_db_t *localdb = alpm_get_localdb(handle);
	register_sync_dbs(handle, syncdbs);

	for(it = alpm_db_get_pkgcache(localdb); it; it=alpm_list_next(it)){
		pkg = it->data;
		spkg = alpm_sync_get_new_version(pkg, alpm_get_syncdbs(handle));
		if(spkg != NULL) {
			/****** LEAK? ******/
			upd_package* upkg = (upd_package*)malloc(sizeof(upd_package));
			upkg->name = _strdup(alpm_pkg_get_name(pkg));
			upkg->loc_version = _strdup(alpm_pkg_get_version(pkg));
			upkg->rem_version = _strdup(alpm_pkg_get_version(spkg));
			ret = alpm_list_add(ret, upkg);
			/****** LEAK? ******/
		}
	}

	alpm_release(handle);
	return ret;
}

alpm_list_t* get_foreign(alpm_list_t* syncdbs){
	alpm_list_t *it = NULL;
	alpm_list_t *ret = NULL;
	alpm_pkg_t *pkg = NULL;

	alpm_handle_t* handle = create_handle();
	alpm_db_t *localdb = alpm_get_localdb(handle);
	register_sync_dbs(handle, syncdbs);
	for(it = alpm_db_get_pkgcache(localdb); it; it = alpm_list_next(it)) {
		pkg = it->data;
		if(is_foreign(handle, pkg)){
			upd_package* upkg = (upd_package*)malloc(sizeof(upd_package));
			upkg->name = _strdup(alpm_pkg_get_name(pkg));
			upkg->loc_version = _strdup(alpm_pkg_get_version(pkg));
			upkg->rem_version = _strdup("0");
			ret = alpm_list_add(ret, upkg);
		}
	}
	alpm_release(handle);
	return ret;
}

alpm_list_t* get_group_pkgs(const char* group) {
	alpm_list_t* it = NULL;
	alpm_list_t* ret = NULL;
	alpm_group_t* grp = NULL;
	alpm_pkg_t* pkg = NULL;

	alpm_handle_t* handle = create_handle();
	alpm_db_t* localdb = alpm_get_localdb(handle);
	grp = alpm_db_get_group(localdb, group);
	for(it = grp->packages; it; it = alpm_list_next(it)){
		pkg = it->data;
		ret = alpm_list_add(ret, _strdup(alpm_pkg_get_name(pkg)));
	}
	alpm_release(handle);
	return ret;
}

int sync_dbs(alpm_list_t* dbs, int force){
	alpm_list_t *it;
	int retval = 0;
	int tempret = 0;
	alpm_list_t *db = NULL;
	alpm_handle_t* handle = create_handle();
	register_sync_dbs(handle, dbs);
	tempret = alpm_db_update(handle, dbs, force);
	retval += abs(tempret);
	if(tempret < 0){
		printf("error: %s\n", alpm_strerror(alpm_errno(handle)));
	} else {
		printf("dbs synced successfully: %d\n", retval);
	}
	alpm_release(handle);
	return retval;
}

const char* pkgver(const char* pkgname) {
	alpm_errno_t err;
	alpm_handle_t *handle;
	alpm_pkg_t *pkg;
	alpm_db_t *db;
	const char *retval;

    assert(paths != NULL);
    const char *root = paths->root;
    const char *lib = paths->lib;

	handle = alpm_initialize(root, lib, &err);
	if (handle) {
		db = alpm_get_localdb(handle);
		pkg = alpm_db_get_pkg(db, pkgname);
        retval = _strdup(alpm_pkg_get_version(pkg));
		alpm_release(handle);
		return retval;
	}
	return NULL;
}

