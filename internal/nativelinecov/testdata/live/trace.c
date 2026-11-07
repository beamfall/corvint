// SPDX-License-Identifier: Apache-2.0
#define _GNU_SOURCE
#include <sys/ptrace.h>
#include <sys/wait.h>
#include <unistd.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <errno.h>
static int tgid(pid_t pid) {
 char path[64],line[128];snprintf(path,sizeof(path),"/proc/%d/status",pid);
 FILE *f=fopen(path,"r");if(!f)return -1;int id=-1;
 while(fgets(line,sizeof(line),f))if(sscanf(line,"Tgid:%d",&id)==1)break;
 fclose(f);return id;
}
int main(int argc,char **argv){
 if(argc<3)return 125;
 FILE *f=fopen(argv[1],"w");if(!f)return 125;setbuf(f,NULL);
 pid_t root=fork();if(root<0)return 125;
 if(!root){fclose(f);if(ptrace(PTRACE_TRACEME,0,0,0)<0){perror("PTRACE_TRACEME");_exit(125);}raise(SIGSTOP);execvp(argv[2],argv+2);perror("execvp");_exit(125);}
 int status,result=125;if(waitpid(root,&status,0)!=root||!WIFSTOPPED(status))return 125;
 long options=PTRACE_O_TRACEFORK|PTRACE_O_TRACEVFORK|PTRACE_O_TRACECLONE|PTRACE_O_TRACEEXEC|PTRACE_O_EXITKILL;
 if(ptrace(PTRACE_SETOPTIONS,root,0,options)<0){perror("PTRACE_SETOPTIONS");return 125;}
 fprintf(f,"START %d 0\n",root);ptrace(PTRACE_CONT,root,0,0);
 for(;;){
  pid_t pid=waitpid(-1,&status,__WALL);if(pid<0){if(errno==EINTR)continue;if(errno==ECHILD)break;perror("waitpid");return 125;}
  if(WIFEXITED(status)||WIFSIGNALED(status)){int code=WIFEXITED(status)?WEXITSTATUS(status):128+WTERMSIG(status);fprintf(f,"EXIT %d %d\n",pid,code);if(pid==root)result=code;continue;}
  if(!WIFSTOPPED(status))continue;
  unsigned event=(unsigned)status>>16;int sig=WSTOPSIG(status);
  if(event==PTRACE_EVENT_FORK||event==PTRACE_EVENT_VFORK||event==PTRACE_EVENT_CLONE){unsigned long child=0;if(ptrace(PTRACE_GETEVENTMSG,pid,0,&child)<0)return 125;int parent=tgid(pid), group=tgid((pid_t)child);if(parent<0||group<0)return 125;fprintf(f,"%s %lu %d\n",group==parent?"CLONE":"START",child,parent);}
  if(event==PTRACE_EVENT_EXEC){char path[64],exe[4096];snprintf(path,sizeof(path),"/proc/%d/exe",pid);ssize_t n=readlink(path,exe,sizeof(exe)-1);if(n<0)return 125;exe[n]=0;fprintf(f,"EXEC %d %s\n",pid,exe);}
  if(ptrace(PTRACE_CONT,pid,0,(event||sig==SIGTRAP||sig==SIGSTOP)?0:sig)<0&&errno!=ESRCH){perror("PTRACE_CONT");return 125;}
 }
 fprintf(f,"COMPLETE\n");fclose(f);return result;
}
