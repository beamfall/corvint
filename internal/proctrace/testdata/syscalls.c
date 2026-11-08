/* SPDX-License-Identifier: Apache-2.0 */
#define _GNU_SOURCE
#include <sys/syscall.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <sys/socket.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <netinet/in.h>
#include <linux/openat2.h>
#include <linux/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <sched.h>
#include <signal.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <errno.h>

static void check(long n) { if (n < 0) { perror("fixture"); exit(91); } }
static void readfile(const char *p) {
 char b[64]; int fd=open(p,O_RDONLY); check(fd); check(read(fd,b,sizeof b)); check(close(fd));
}
static int child(void *ignored) { (void)ignored; readfile("child"); _exit(0); }
static void waited(pid_t p) { int status; check(p); check(waitpid(p,&status,0)); if (!WIFEXITED(status) || WEXITSTATUS(status)) exit(92); }
int main(int argc,char **argv) {
 if (argc<2) return 90;
 const char *mode=argv[1];
 if (!strcmp(mode,"calls")) {
  struct open_how how={.flags=O_RDONLY};
  int fd=syscall(SYS_openat2,AT_FDCWD,"data",&how,sizeof how); check(fd); char b[4096]; check(read(fd,b,sizeof b)); close(fd);
  struct statx st; check(syscall(SYS_statx,AT_FDCWD,"stat-data",0,STATX_BASIC_STATS,&st));
  check(syscall(SYS_faccessat2,AT_FDCWD,"access-data",R_OK,0));
  fd=open(".",O_RDONLY|O_DIRECTORY); check(fd); check(syscall(SYS_getdents64,fd,b,sizeof b)); close(fd);
  if (open("missing",O_RDONLY)!=-1 || errno!=ENOENT) return 93;
  fd=open("made",O_WRONLY|O_CREAT|O_EXCL,0600); check(fd); check(write(fd,"new",3)); close(fd);
  check(syscall(SYS_renameat2,AT_FDCWD,"made",AT_FDCWD,"replaced",0)); readfile("replaced");
  int sock=socket(AF_INET,SOCK_STREAM|SOCK_NONBLOCK,0); check(sock);
  struct sockaddr_in addr={.sin_family=AF_INET,.sin_addr.s_addr=htonl(INADDR_LOOPBACK),.sin_port=htons(1)};
  syscall(SYS_connect,sock,&addr,sizeof addr); close(sock);
  pid_t p=fork(); if (p==0) { char *args[]={"./exec-target","child",NULL}; char *env[]={NULL}; syscall(SYS_execveat,AT_FDCWD,args[0],args,env,0); _exit(94); } waited(p);
 } else if (!strcmp(mode,"child")) { readfile("child");
 } else if (!strcmp(mode,"fork")) { pid_t p=syscall(SYS_fork); if (p==0) child(NULL); waited(p);
 } else if (!strcmp(mode,"vfork")) { pid_t p=vfork(); if (p==0) { char *args[]={argv[0],"child",NULL}; char *env[]={NULL}; execve(argv[0],args,env); _exit(94); } waited(p);
 } else if (!strcmp(mode,"clone")) {
  char *stack=malloc(65536); if (!stack) return 95;
  waited(clone(child,stack+65536,CLONE_VM|SIGCHLD,NULL)); free(stack);
 } else if (!strcmp(mode,"exec")) { char *args[]={argv[0],"child",NULL}; char *env[]={NULL}; execve(argv[0],args,env); return 94;
 } else if (!strcmp(mode,"setuid")) { char *args[]={"./privileged","child",NULL}; char *env[]={NULL}; execve(args[0],args,env); return 94;
 } else if (!strcmp(mode,"writes")) {
  int fd=open("created",O_CREAT|O_EXCL|O_WRONLY,0600); check(fd); check(write(fd,"abc",3)); close(fd); readfile("created");
  fd=open("append",O_APPEND|O_WRONLY); check(fd); check(write(fd,"abc",3)); close(fd); readfile("append");
  fd=open("overwrite",O_WRONLY); check(fd); check(pwrite(fd,"z",1,1)); close(fd); readfile("overwrite");
  fd=open("truncate",O_WRONLY|O_TRUNC); check(fd); close(fd); readfile("truncate");
 } else if (!strcmp(mode,"write-open")) {
  int fd=open("data",O_WRONLY); check(fd); check(close(fd));
 } else if (!strcmp(mode,"open-write-only")) {
  if (argc!=3) return 90;
  int fd=syscall(SYS_open,"data",strtoul(argv[2],NULL,0),0600);
  if (fd<0) return errno==ENOENT ? 1 : 2;
  check(close(fd));
 } else if (!strcmp(mode,"write-dependency")) {
  if (argc!=5) return 90;
  const char *name="data";
  if (!strcmp(argv[4],"owned")) {
   name="made";
   int fd=open(name,O_CREAT|O_EXCL|O_WRONLY,0600); check(fd); close(fd);
  }
  if (!strcmp(argv[2],"truncate")) return syscall(SYS_truncate,name,0)==0 ? 0 : 1;
  int fd=syscall(SYS_open,name,strtoul(argv[3],NULL,0),0600);
  if (fd<0) return 1;
  close(fd);
 } else if (!strcmp(mode,"path-create")) {
  int fd=open("data",O_PATH|O_CREAT|O_EXCL|O_TRUNC|O_RDWR,0600); check(fd); close(fd);
  unsigned char b; fd=open("data",O_RDONLY); check(fd); check(read(fd,&b,1)); close(fd); return b;
 } else if (!strcmp(mode,"open-future")) {
  int fd=syscall(SYS_open,"made",O_CREAT|O_EXCL|O_RDWR|0x40000000,0600); check(fd);
  check(write(fd,"x",1)); close(fd); readfile("made");
 } else if (!strcmp(mode,"shared-memory")) {
  if (argc!=4) return 90;
  int fd=open(argv[2],O_RDWR); check(fd);
  int flags=atoi(argv[3]);
  unsigned char *memory=mmap(NULL,4096,PROT_READ|PROT_WRITE,flags,fd,0);
  if (memory==MAP_FAILED) return 96;
  unsigned char response=memory[0]; memory[1]=response+1;
  check(munmap(memory,4096)); close(fd); return response;
 } else if (!strcmp(mode,"cwd-byte")) {
  unsigned char b; int fd=open("data",O_RDONLY); check(fd); check(read(fd,&b,1)); close(fd); return b;
 } else if (!strcmp(mode,"symlink")) { readfile("link");
 } else if (!strcmp(mode,"outside")) { if (argc!=3) return 90; readfile(argv[2]);
 } else if (!strcmp(mode,"utimensat")) {
  return syscall(SYS_utimensat,AT_FDCWD,"data",NULL,0)==0 ? 0 : 1;
 } else if (!strcmp(mode,"sigqueue")) {
  siginfo_t info={.si_signo=0,.si_code=SI_QUEUE};
  check(syscall(SYS_rt_sigqueueinfo,getppid(),0,&info));
 } else if (!strcmp(mode,"future")) { syscall(0x3fffffff);
 } else if (!strcmp(mode,"rename-parent")) {
  long result;
  if (argc!=3) return 90;
  if (!strcmp(argv[2],"rename")) result=syscall(SYS_rename,"data","missing/out");
  else if (!strcmp(argv[2],"renameat")) result=syscall(SYS_renameat,AT_FDCWD,"data",AT_FDCWD,"missing/out");
  else result=syscall(SYS_renameat2,AT_FDCWD,"data",AT_FDCWD,"missing/out",0);
  return result==0 ? 0 : (errno==ENOENT ? 1 : 2);
 } else if (!strcmp(mode,"rename-symlink")) {
  if (argc!=3) return 90;
  int fd=open("tmp",O_CREAT|O_EXCL|O_WRONLY,0600); check(fd); check(write(fd,"new",3)); check(close(fd));
  if (!strcmp(argv[2],"rename")) check(syscall(SYS_rename,"tmp","dest"));
  else if (!strcmp(argv[2],"renameat")) check(syscall(SYS_renameat,AT_FDCWD,"tmp",AT_FDCWD,"dest"));
  else check(syscall(SYS_renameat2,AT_FDCWD,"tmp",AT_FDCWD,"dest",0));
  readfile("data");
 } else if (!strcmp(mode,"lookup")) {
  int fd=open("alias",O_RDONLY);
  if (fd<0) return errno==ENOENT ? 1 : 2;
  check(close(fd));
 } else if (!strcmp(mode,"byte")) {
  unsigned char b; int fd=open("alias/data",O_RDONLY); check(fd); check(read(fd,&b,1)); close(fd); return b;
 } else if (!strcmp(mode,"unknown")) { syscall(SYS_open_by_handle_at,-1,NULL,0);
 } else if (!strcmp(mode,"malformed")) { syscall(SYS_openat2,AT_FDCWD,"data",(void *)1,24);
 } else if (!strcmp(mode,"pause")) { pause();
 } else if (!strcmp(mode,"signal")) { raise(SIGTERM);
 } else return 90;
 return 0;
}
