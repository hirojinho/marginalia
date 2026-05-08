#!/bin/bash
# Sync JSON study plans to markdown files for the study app agent context

set -e

PLANS_DIR="/workspace/study-app/data/plans"
MEMORY_DIR="/workspace/study-app/memory/courses"

convert_plan() {
  local json_file="$1"
  local out_file="$2"
  
  node -e "
    const fs = require('fs');
    const data = JSON.parse(fs.readFileSync('$json_file', 'utf8'));
    let md = '# ' + data.name + '\n\n';
    let n = 0;
    for (const phase of data.phases) {
      md += '## ' + phase.title + '\n\n';
      if (phase.clusters) {
        for (const cluster of phase.clusters) {
          md += '### ' + cluster.title + '\n\n';
          for (const task of cluster.tasks) {
            n++;
            const status = task.done ? '[x]' : '[ ]';
            md += '- ' + status + ' **' + n + '** ' + task.title.replace(/\*\*/g, '') + '\n';
            if (task.notes) md += '  ' + task.notes.split('\n').join('\n  ') + '\n';
          }
          md += '\n';
        }
      } else if (phase.tasks) {
        for (const task of phase.tasks) {
          n++;
          const status = task.done ? '[x]' : '[ ]';
          md += '- ' + status + ' **' + n + '** ' + task.title.replace(/\*\*/g, '') + '\n';
          if (task.notes) md += '  ' + task.notes.split('\n').join('\n  ') + '\n';
        }
        md += '\n';
      }
    }
    fs.writeFileSync('$out_file', md);
    console.log('Written ' + '$out_file' + ' with ' + n + ' tasks');
  "
}

# Convert each plan
convert_plan "$PLANS_DIR/ce297.json" "$MEMORY_DIR/ce297/study-plan.md"
convert_plan "$PLANS_DIR/ddia.json" "$MEMORY_DIR/ddia/study-plan.md"
convert_plan "$PLANS_DIR/software-arch.json" "$MEMORY_DIR/software-arch/study-plan.md"
convert_plan "$PLANS_DIR/thesis.json" "/workspace/study-app/memory/thesis/study-plan.md"

echo "All plans synced"
