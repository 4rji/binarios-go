#include <stdio.h>
#include <unistd.h> 
int main(int argc, char ** argv) {
  FILE * fp = fopen("/tmp/log.txt", "w+");
  while (1) {
    sleep(1);
    fprintf(fp, "I always wanted to be a DAEMON!\n");
    fprintf(fp, "  |\\___/|\n");
    fprintf(fp, " /       \\\n");
    fprintf(fp, "|    /\\__/|\n");
    fprintf(fp, "||\\  <.><.>\n");
    fprintf(fp, "| _     > )\n");
    fprintf(fp, " \\   /----\n");
    fprintf(fp, "  |   -\\/\n");
    fprintf(fp, " /     \\\n\n");
    fprintf(fp, "Wait, something is not right...\n");
    fflush(fp);
  }
  fclose(fp);
  return 0;
}
