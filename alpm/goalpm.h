#ifndef GOALPM_H
#define GOALPM_H

#include <alpm.h>
#include <alpm_list.h>

typedef struct syncdb {
	char* name;
	alpm_list_t* servers;
} syncdb;

typedef struct upd_package {
	char* name;
	char* loc_version;
	char* rem_version;
} upd_package;

syncdb* new_syncdb(char*);
void init_paths(char*, char*);
void goalpm_cleanup();
void add_server_to_syncdb(syncdb*, char*);
void free_syncdb_list(alpm_list_t*);
void dump_syncdb_list(alpm_list_t*);
void free_pkg_list(alpm_list_t*);
int sync_dbs(alpm_list_t*, int);

alpm_list_t* get_updates(alpm_list_t*);
alpm_list_t* get_foreign(alpm_list_t*);
alpm_list_t* get_group_pkgs(const char*);

const char* pkgver(const char* pkgname);
#endif
