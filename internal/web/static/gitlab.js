/* Git Recovery Lab — an animated, command-driven git repair minigame.
 *
 * The engine is deliberately DOM-free so it can be exercised in Node:
 *   const { SCENARIOS, initScenario, execute } = require("./gitlab.js");
 * Every scenario has a `demo` command per step, which is what the tests run
 * to prove the mission is completable.
 */
(function (root) {
  "use strict";

  var STORAGE_KEY = "mallard.gitlab.v1";
  var MAX_STARS = 3;

  function commit(id, msg, parents, lane) {
    return { id: id, msg: msg, parents: parents || [], lane: lane || 0 };
  }

  function short(id) {
    return String(id).slice(0, 7);
  }

  function headSha(s) {
    return s.head.branch ? s.branches[s.head.branch] : s.head.sha;
  }

  function findBySha(s, sha) {
    if (!sha) return null;
    var needle = String(sha).toLowerCase();
    for (var i = 0; i < s.commits.length; i++) {
      var id = s.commits[i].id.toLowerCase();
      if (id === needle || id.indexOf(needle) === 0) return s.commits[i];
    }
    return null;
  }

  function newId(s) {
    s.seq += 1;
    return (s.seq).toString(16).padStart(7, "0");
  }

  function addCommit(s, msg, parents, lane) {
    var c = commit(newId(s), msg, parents, lane);
    s.commits.push(c);
    return c;
  }

  function reachable(s, sha, seen) {
    seen = seen || {};
    if (!sha || seen[sha]) return seen;
    seen[sha] = true;
    var c = findBySha(s, sha);
    if (!c) return seen;
    for (var i = 0; i < c.parents.length; i++) {
      reachable(s, c.parents[i], seen);
    }
    return seen;
  }

  function tipSet(s) {
    var tips = Object.keys(s.branches).map(function (k) { return s.branches[k]; });
    Object.keys(s.remote).forEach(function (k) { tips.push(s.remote[k]); });
    tips.push(headSha(s));
    return tips;
  }

  function reachableFromTips(s) {
    var out = {};
    tipSet(s).forEach(function (tip) {
      var set = reachable(s, tip, {});
      Object.keys(set).forEach(function (sha) { out[sha] = true; });
    });
    return out;
  }

  function counts(s, branch) {
    var local = s.branches[branch];
    var remoteName = (s.upstream && s.upstream[branch]) || branch;
    var remote = s.remote[remoteName];
    if (!local || !remote) return { state: "no-remote", ahead: 0, behind: 0 };
    if (local === remote) return { state: "same", ahead: 0, behind: 0 };
    var localSet = reachable(s, local, {});
    var remoteSet = reachable(s, remote, {});
    var ahead = Object.keys(localSet).filter(function (sha) { return !remoteSet[sha]; }).length;
    var behind = Object.keys(remoteSet).filter(function (sha) { return !localSet[sha]; }).length;
    if (ahead > 0 && behind > 0) return { state: "diverged", ahead: ahead, behind: behind };
    if (ahead > 0) return { state: "ahead", ahead: ahead, behind: 0 };
    return { state: "behind", ahead: 0, behind: behind };
  }

  function refsFor(s, sha) {
    var refs = [];
    Object.keys(s.branches).forEach(function (name) {
      if (s.branches[name] === sha) refs.push(name);
    });
    Object.keys(s.remote).forEach(function (name) {
      if (s.remote[name] === sha) refs.push("origin/" + name);
    });
    if (headSha(s) === sha) {
      refs.unshift(s.head.branch ? "HEAD -> " + s.head.branch : "HEAD");
    }
    return refs;
  }

  function statusLines(s) {
    var out = [];
    if (s.rebase) {
      out.push("interactive rebase in progress; onto " + short(s.rebase.onto));
      out.push("You are currently rebasing branch '" + s.rebase.branch + "' on '" + short(s.rebase.onto) + "'.");
      out.push('  (fix conflicts and run "git rebase --continue")');
      out.push('  (use "git rebase --abort" to check out the original branch)');
      out.push("");
    }
    if (s.head.branch) {
      out.push("On branch " + s.head.branch);
      var c = counts(s, s.head.branch);
      var upstream = (s.upstream && s.upstream[s.head.branch]) || s.head.branch;
      if (c.state === "same") out.push("Your branch is up to date with 'origin/" + upstream + "'.");
      if (c.state === "ahead") out.push("Your branch is ahead of 'origin/" + upstream + "' by " + c.ahead + " commit" + (c.ahead === 1 ? "" : "s") + ".");
      if (c.state === "behind") out.push("Your branch is behind 'origin/" + upstream + "' by " + c.behind + " commit" + (c.behind === 1 ? "" : "s") + ", and can be fast-forwarded.");
      if (c.state === "diverged") out.push("Your branch and 'origin/" + upstream + "' have diverged, and have " + c.ahead + " and " + c.behind + " different commits each, respectively.");
    } else {
      out.push("HEAD detached " + (s.detachedFrom ? "from " + short(s.detachedFrom) : "at " + short(headSha(s))));
      out.push('You are in "detached HEAD" state. You can look around and make commits,');
      out.push("but no branch points at them unless you create one now.");
    }

    if (s.conflict) {
      out.push("");
      out.push("You have unmerged paths.");
      out.push("  (fix conflicts and run \"git rebase --continue\")");
      out.push("Unmerged paths:");
      out.push("  (use \"git add <file>...\" to mark resolution)");
      out.push("\tboth modified:   " + s.conflict.file);
    }

    var staged = s.files.staged || [];
    var modified = s.files.modified || [];
    var untracked = s.files.untracked || [];
    if (staged.length) {
      out.push("");
      out.push("Changes to be committed:");
      staged.forEach(function (f) { out.push("\tnew file:   " + f); });
    }
    if (modified.length) {
      out.push("");
      out.push("Changes not staged for commit:");
      modified.forEach(function (f) { out.push("\tmodified:   " + f); });
    }
    if (untracked.length) {
      out.push("");
      out.push("Untracked files:");
      untracked.forEach(function (f) { out.push("\t" + f); });
    }
    if (!staged.length && !modified.length && !untracked.length && !s.conflict) {
      out.push("nothing to commit, working tree clean");
    }
    return out;
  }

  function logLines(s, all, limit) {
    var tips = all ? tipSet(s) : [headSha(s)];
    var set = {};
    tips.forEach(function (tip) {
      var r = reachable(s, tip, {});
      Object.keys(r).forEach(function (sha) { set[sha] = true; });
    });
    var list = s.commits.filter(function (c) { return set[c.id]; });
    if (limit && limit > 0) list = list.slice(-limit);
    return list.slice().reverse().map(function (c) {
      var refs = refsFor(s, c.id);
      return "* " + short(c.id) + " " + c.msg + (refs.length ? " (" + refs.join(", ") + ")" : "");
    });
  }

  function reflogLines(s) {
    return s.reflog.map(function (entry, i) {
      return short(entry.sha) + " HEAD@{" + i + "}: " + entry.action;
    });
  }

  function branchLines(s) {
    return Object.keys(s.branches).map(function (name) {
      var active = s.head.branch === name ? "*" : " ";
      var c = findBySha(s, s.branches[name]);
      return active + " " + name + " " + short(s.branches[name]) + " " + (c ? c.msg : "");
    });
  }

  function diffLines(s) {
    if (!s.conflict) return ["(no unstaged diff in the lab state)"];
    return [
      "diff --cc " + s.conflict.file,
      "@@ -12,7 +12,11 @@ func Search(ctx context.Context, q string) ([]Result, error) {",
      "++<<<<<<< HEAD",
      "+ \trows, err := db.Query(ctx, legacySearchSQL, q)",
      "++=======",
      "+ \trows, err := db.Query(ctx, rankedSearchSQL, q)",
      "++>>>>>>> " + short(headSha(s)) + " (Add search endpoint)"
    ];
  }

  function observeLines(s, input) {
    var cmd = input.replace(/^git\s+/, "");
    if (/^status\b/.test(cmd)) return statusLines(s);
    if (/^reflog\b/.test(cmd)) return reflogLines(s);
    if (/^branch\b/.test(cmd)) return branchLines(s);
    if (/^diff\b/.test(cmd)) return diffLines(s);
    if (/^log\b/.test(cmd)) {
      var limit = 0;
      var m = cmd.match(/\s-(\d+)/);
      if (m) limit = parseInt(m[1], 10);
      return logLines(s, /--all/.test(cmd), limit);
    }
    return null;
  }

  function cmdLine(text) {
    return { kind: "cmd", text: "$ " + text };
  }

  function outLines(lines) {
    return (lines || []).map(function (text) {
      return typeof text === "string" ? { kind: "out", text: text } : text;
    });
  }

  /* ------------------------------------------------------------------ */
  /* Scenarios                                                          */
  /* ------------------------------------------------------------------ */

  var SCENARIOS = [
    {
      id: "hotfix-on-main",
      title: "The hotfix landed on main",
      tag: "MISPLACED COMMIT",
      difficulty: 1,
      brief: "You fixed a nil dereference directly on main. Main is protected and must match origin, but the fix cannot be lost.",
      objectives: [
        "Move the fix onto a branch called hotfix/auth-nil",
        "Restore main to origin/main",
        "Publish the hotfix branch"
      ],
      note: {
        title: "Create first, reset second",
        body: "A branch is just another name for a commit. Create it while HEAD still points at the fix, then switch main back and reset it. Resetting before branching is the classic way to lose the commit."
      },
      initial: function () {
        return {
          commits: [
            commit("c1aaaaa", "Initial commit", [], 0),
            commit("c2bbbbb", "Add login handler", ["c1aaaaa"], 0),
            commit("c3ccccc", "Bump dependencies", ["c2bbbbb"], 0),
            commit("c4ddddd", "Fix nil deref in auth", ["c3ccccc"], 0)
          ],
          branches: { main: "c4ddddd" },
          remote: { main: "c3ccccc" },
          head: { branch: "main" },
          upstream: { main: "main" },
          files: { staged: [], modified: [], untracked: [] },
          rebase: null,
          conflict: null,
          origHead: "c4ddddd",
          reflog: [
            { sha: "c4ddddd", action: "commit: Fix nil deref in auth" },
            { sha: "c3ccccc", action: "commit: Bump dependencies" },
            { sha: "c2bbbbb", action: "commit: Add login handler" },
            { sha: "c1aaaaa", action: "commit (initial): Initial commit" }
          ]
        };
      },
      steps: [
        {
          title: "Inspect the divergence",
          demo: "git status",
          check: "Look at how far main is ahead of origin.",
          expect: "main is ahead of origin/main by exactly one commit.",
          hint: "git status compares your branch with its upstream.",
          blocks: [{
            re: /^git reset --hard/,
            message: "Do not reset yet. HEAD is on main and the fix is only named by main. Create the hotfix branch first."
          }],
          accept: [{
            re: /^git (status|log|branch)\b.*$/,
            run: function (s, input) { return observeLines(s, input); }
          }],
          note: {
            title: "Read before you rewrite",
            body: "The one-commit lead is the fix. Everything that follows exists to keep that commit while moving the main ref."
          }
        },
        {
          title: "Name the fix before moving anything",
          demo: "git switch -c hotfix/auth-nil",
          check: "Create a branch at the current commit.",
          expect: "HEAD now points at hotfix/auth-nil, still on the fix commit.",
          hint: "git switch -c <name> creates the branch and checks it out in one step.",
          blocks: [{
            re: /^git reset --hard/,
            message: "Do not reset yet. HEAD is on main; a hard reset would erase the fix. Create the branch first."
          }],
          accept: [{
            re: /^git (switch -c|checkout -b) hotfix\/auth-nil$/,
            run: function (s) {
              s.branches["hotfix/auth-nil"] = headSha(s);
              s.head = { branch: "hotfix/auth-nil" };
              return ["Switched to a new branch 'hotfix/auth-nil'"];
            }
          }],
          note: {
            title: "Branch as a bookmark",
            body: "No files were copied. Git wrote a 41-byte ref that names the same commit. The fix now has two names: main and hotfix/auth-nil."
          }
        },
        {
          title: "Move main back",
          demo: "git switch main",
          check: "Check out the branch that must be restored.",
          expect: "HEAD is on main again, still at the fix commit.",
          hint: "Switch to main before resetting it.",
          blocks: [{
            re: /^git reset --hard origin\/main$/,
            message: "You are still on hotfix/auth-nil. Resetting here would move the hotfix branch and orphan the commit. Switch to main first."
          }],
          accept: [{
            re: /^git (switch|checkout) main$/,
            run: function (s) {
              s.head = { branch: "main" };
              return ["Switched to branch 'main'", "Your branch is ahead of 'origin/main' by 1 commit."];
            }
          }]
        },
        {
          title: "Reset main to the remote",
          demo: "git reset --hard origin/main",
          check: "Make main match origin/main exactly.",
          expect: "main points at the dependency bump; the hotfix branch still holds the fix.",
          hint: "git reset --hard origin/main moves the current branch and the working tree.",
          blocks: [{
            re: /^git reset --hard(?! origin\/main$)/,
            message: "Use origin/main as the target. Any other commit leaves main in the wrong place."
          }],
          accept: [{
            re: /^git reset --hard origin\/main$/,
            run: function (s) {
              s.branches.main = s.remote.main;
              s.reflog.unshift({ sha: s.remote.main, action: "reset: moving to origin/main" });
              return ["HEAD is now at " + short(s.remote.main) + " Bump dependencies"];
            }
          }],
          note: {
            title: "Hard reset has a narrow use",
            body: "reset --hard is safe here only because the commit is already named by another branch. On a shared branch with no backup ref, it destroys work."
          }
        },
        {
          title: "Publish the hotfix",
          demo: "git push -u origin hotfix/auth-nil",
          check: "Send the hotfix branch to origin and set its upstream.",
          expect: "origin/hotfix/auth-nil appears next to the fix commit.",
          hint: "git push -u origin <branch> publishes and records the upstream in one command.",
          accept: [{
            re: /^git push( -u)? origin hotfix\/auth-nil$/,
            run: function (s) {
              s.remote["hotfix/auth-nil"] = s.branches["hotfix/auth-nil"];
              s.upstream["hotfix/auth-nil"] = "hotfix/auth-nil";
              return [
                "Enumerating objects: 7, done.",
                "To github.com:acme/service.git",
                " * [new branch]      hotfix/auth-nil -> hotfix/auth-nil",
                "branch 'hotfix/auth-nil' set up to track 'origin/hotfix/auth-nil'."
              ];
            }
          }]
        },
        {
          title: "Verify the final graph",
          demo: "git log --oneline --graph --decorate --all -4",
          check: "Confirm main and the hotfix branch now point at different commits.",
          expect: "main matches origin/main; hotfix/auth-nil contains the fix.",
          hint: "Add --all so local and remote refs both appear.",
          accept: [{
            re: /^git (log|branch)\b.*$/,
            run: function (s, input) { return observeLines(s, input); }
          }]
        }
      ]
    },

    {
      id: "amend-unpushed",
      title: "The commit is missing a file",
      tag: "AMEND",
      difficulty: 1,
      brief: "You committed the auth handler but forgot the test file, and the message undersells the change. Nothing has been pushed.",
      objectives: [
        "Stage the forgotten test file",
        "Rewrite the unpushed commit with the file and a better message"
      ],
      note: {
        title: "Amend rewrites, it does not append",
        body: "git commit --amend creates a new commit and moves the branch. The old commit stays in the reflog until garbage collection. Safe before push, hostile after."
      },
      initial: function () {
        return {
          commits: [
            commit("a1aaaaa", "Initial commit", [], 0),
            commit("a2bbbbb", "Add auth handler", ["a1aaaaa"], 0)
          ],
          branches: { main: "a2bbbbb" },
          remote: { main: "a1aaaaa" },
          head: { branch: "main" },
          upstream: { main: "main" },
          files: { staged: [], modified: [], untracked: ["internal/auth/handler_test.go"] },
          rebase: null,
          conflict: null,
          origHead: "a2bbbbb",
          reflog: [
            { sha: "a2bbbbb", action: "commit: Add auth handler" },
            { sha: "a1aaaaa", action: "commit (initial): Initial commit" }
          ]
        };
      },
      steps: [
        {
          title: "Find the forgotten file",
          demo: "git status",
          check: "Identify what is untracked.",
          expect: "handler_test.go is untracked; the commit is already recorded without it.",
          hint: "git status separates staged, modified, and untracked files.",
          accept: [{
            re: /^git (status|log|diff)\b.*$/,
            run: function (s, input) { return observeLines(s, input); }
          }]
        },
        {
          title: "Stage the test file",
          demo: "git add internal/auth/handler_test.go",
          check: "Add the forgotten file to the index.",
          expect: "The file moves to Changes to be committed.",
          hint: "git add <path> stages one file. git add -A stages everything.",
          accept: [{
            re: /^git add (internal\/auth\/handler_test\.go|\.|-A|--all)$/,
            run: function (s) {
              s.files.untracked = s.files.untracked.filter(function (f) { return f !== "internal/auth/handler_test.go"; });
              s.files.staged = ["internal/auth/handler_test.go"];
              return [];
            }
          }]
        },
        {
          title: "Amend the commit",
          demo: 'git commit --amend -m "Add auth handler with tests"',
          check: "Fold the staged file into the previous commit and fix the message.",
          expect: "A new commit hash replaces the old tip; the parent does not change.",
          hint: "git commit --amend --no-edit keeps the message; -m replaces it.",
          blocks: [{
            re: /^git push/,
            message: "Push after the amend. Pushing now would publish the incomplete commit first."
          }],
          accept: [{
            re: /^git commit --amend(?: --no-edit| -m .+)?$/,
            run: function (s, input) {
              var old = findBySha(s, headSha(s));
              var message = old.msg;
              var m = input.match(/-m "([^"]+)"/);
              if (m) message = m[1];
              s.commits = s.commits.filter(function (c) { return c.id !== old.id; });
              var amended = addCommit(s, message, old.parents.slice(), old.lane);
              s.branches.main = amended.id;
              s.files.staged = [];
              s.files.modified = [];
              s.reflog.unshift({ sha: amended.id, action: "commit (amend): " + message });
              return [
                "[main " + short(amended.id) + "] " + message,
                " 1 file changed, 24 insertions(+)",
                " create mode 100644 internal/auth/handler_test.go"
              ];
            }
          }],
          note: {
            title: "The old hash is gone from the branch",
            body: "The commit object still exists and the reflog names it. That is why an accidental amend is recoverable with git reflog + git reset --hard."
          }
        },
        {
          title: "Confirm the history",
          demo: "git log --oneline -2",
          check: "Verify the new tip contains both files.",
          expect: "One commit still, with the corrected message; the parent is unchanged.",
          hint: "git log --stat shows which files the commit now contains.",
          accept: [{
            re: /^git (log|status|diff)\b.*$/,
            run: function (s, input) { return observeLines(s, input); }
          }]
        }
      ]
    },

    {
      id: "rebase-conflict",
      title: "Push rejected: histories diverged",
      tag: "REBASE",
      difficulty: 3,
      brief: "A teammate pushed two commits while you built the search endpoint. The push is rejected and the pull wants a merge. Rebase onto origin/main and resolve the conflict.",
      objectives: [
        "Fetch the remote changes",
        "Rebase your commit onto origin/main",
        "Resolve the conflict and finish the rebase",
        "Push the rewritten history"
      ],
      note: {
        title: "Rebase replays, merge joins",
        body: "A rebase takes your commits, rewinds to the new base, and replays them one by one. Conflicts appear per commit. Resolve, stage, continue. If it goes wrong, git rebase --abort restores the starting point."
      },
      initial: function () {
        return {
          commits: [
            commit("b1aaaaa", "Initial commit", [], 0),
            commit("b2bbbbb", "Update dependencies", ["b1aaaaa"], 0),
            commit("b3ccccc", "Fix CI cache", ["b2bbbbb"], 0),
            commit("b4ddddd", "Add search endpoint", ["b1aaaaa"], 1)
          ],
          branches: { main: "b4ddddd" },
          remote: { main: "b3ccccc" },
          head: { branch: "main" },
          upstream: { main: "main" },
          files: { staged: [], modified: [], untracked: [] },
          rebase: null,
          conflict: null,
          origHead: "b4ddddd",
          reflog: [
            { sha: "b4ddddd", action: "commit: Add search endpoint" },
            { sha: "b1aaaaa", action: "commit (initial): Initial commit" }
          ]
        };
      },
      steps: [
        {
          title: "Refresh the remote refs",
          demo: "git fetch origin",
          check: "Download the teammate's commits without touching your branch.",
          expect: "origin/main advances to Fix CI cache; your working tree is untouched.",
          hint: "fetch updates remote-tracking refs; it never merges.",
          accept: [{
            re: /^git fetch( origin)?$/,
            run: function () {
              return ["From github.com:acme/service", "   b1aaaaa..b3ccccc  main -> origin/main"];
            }
          }]
        },
        {
          title: "Confirm the divergence",
          demo: "git status",
          check: "See how many commits each side has.",
          expect: "Your branch and origin/main have diverged: 1 local, 2 remote.",
          hint: "git status names the upstream branch and the divergence counts.",
          accept: [{
            re: /^git (status|log|branch)\b.*$/,
            run: function (s, input) { return observeLines(s, input); }
          }]
        },
        {
          title: "Start the rebase",
          demo: "git pull --rebase origin main",
          check: "Replay your commit on top of origin/main.",
          expect: "Git stops on a conflict in internal/search/handler.go.",
          hint: "git pull --rebase origin main is fetch + rebase in one command.",
          blocks: [{
            re: /^git merge\b/,
            message: "A merge would work, but the mission is to keep history linear. Use git pull --rebase origin main."
          }],
          accept: [{
            re: /^git (pull --rebase( origin main)?|rebase origin\/main)$/,
            run: function (s) {
              s.rebase = { onto: s.remote.main, branch: "main" };
              s.conflict = { file: "internal/search/handler.go", resolved: false };
              return [
                "Successfully rebased and updated refs/heads/main.",
                "Auto-merging internal/search/handler.go",
                "CONFLICT (content): Merge conflict in internal/search/handler.go",
                "error: could not apply " + short(s.branches.main) + "... Add search endpoint",
                "hint: Resolve all conflicts manually, mark them as resolved with",
                "hint: \"git add <conflicted_files>\", then run \"git rebase --continue\"."
              ];
            }
          }]
        },
        {
          title: "Inspect the conflict",
          demo: "git status",
          check: "Find which file needs a decision.",
          expect: "The rebase is paused; handler.go is listed as both modified.",
          hint: "git status during a rebase tells you the exact next command.",
          accept: [{
            re: /^git (status|diff)\b.*$/,
            run: function (s, input) { return observeLines(s, input); }
          }]
        },
        {
          title: "Mark the conflict resolved",
          demo: "git add internal/search/handler.go",
          check: "Stage the file after choosing the correct code.",
          expect: "The unmerged path disappears; the rebase is still in progress.",
          hint: "Editing the file is done outside git; git add records your decision.",
          blocks: [{
            re: /^git commit\b/,
            message: "During a rebase you do not commit manually. Stage the file and run git rebase --continue."
          }],
          accept: [{
            re: /^git add internal\/search\/handler\.go$/,
            run: function (s) {
              s.conflict = null;
              return [];
            }
          }]
        },
        {
          title: "Finish the replay",
          demo: "git rebase --continue",
          check: "Apply the remaining commits and close the rebase.",
          expect: "Your commit gets a new hash and now sits directly on origin/main.",
          hint: "git rebase --continue resumes after the index is clean.",
          accept: [{
            re: /^git rebase --continue$/,
            run: function (s) {
              var old = findBySha(s, s.branches.main);
              if (old) s.commits = s.commits.filter(function (c) { return c.id !== old.id; });
              var rebased = addCommit(s, "Add search endpoint", [s.remote.main], 1);
              s.branches.main = rebased.id;
              s.rebase = null;
              s.reflog.unshift({ sha: rebased.id, action: "rebase (finish): returning to refs/heads/main" });
              return ["Successfully rebased and updated refs/heads/main."];
            }
          }],
          note: {
            title: "New hash, same change",
            body: "The commit content is the same but its parent changed, so its hash changed. Anything that referenced the old hash must be updated."
          }
        },
        {
          title: "Push the linear history",
          demo: "git push",
          check: "Publish the rebased commit.",
          expect: "origin/main fast-forwards to the replayed commit.",
          hint: "After a rebase of unpublished work, a normal push is enough.",
          blocks: [{
            re: /^git push --force/,
            message: "Force-pushing rewrites shared history. Your work was never published, so a normal push fast-forwards."
          }],
          accept: [{
            re: /^git push( origin main)?$/,
            run: function (s) {
              s.remote.main = s.branches.main;
              return [
                "To github.com:acme/service.git",
                "   " + short(s.remote.main) + ".." + short(s.branches.main) + "  main -> main"
              ];
            }
          }]
        },
        {
          title: "Verify the graph",
          demo: "git log --oneline --graph --decorate -4",
          check: "Confirm a single line of history from origin/main.",
          expect: "No merge commit; your commit is the tip with origin/main on it.",
          hint: "A linear graph is the proof the rebase worked.",
          accept: [{
            re: /^git (log|status|branch)\b.*$/,
            run: function (s, input) { return observeLines(s, input); }
          }]
        }
      ]
    },

    {
      id: "revert-pushed",
      title: "The bad commit is already pushed",
      tag: "REVERT",
      difficulty: 2,
      brief: "Your caching change broke invalidation and two teammates already pulled main. The commit must be undone without rewriting shared history.",
      objectives: [
        "Inspect the published history",
        "Create a revert commit",
        "Push the revert"
      ],
      note: {
        title: "Revert is a forward fix",
        body: "git revert creates a new commit that applies the inverse patch. History stays intact, so everyone who already pulled can fast-forward. Resetting a shared branch forces everyone else to recover manually."
      },
      initial: function () {
        return {
          commits: [
            commit("d1aaaaa", "Initial commit", [], 0),
            commit("d2bbbbb", "Add response caching", ["d1aaaaa"], 0)
          ],
          branches: { main: "d2bbbbb" },
          remote: { main: "d2bbbbb" },
          head: { branch: "main" },
          upstream: { main: "main" },
          files: { staged: [], modified: [], untracked: [] },
          rebase: null,
          conflict: null,
          origHead: "d2bbbbb",
          reflog: [
            { sha: "d2bbbbb", action: "commit: Add response caching" },
            { sha: "d1aaaaa", action: "commit (initial): Initial commit" }
          ]
        };
      },
      steps: [
        {
          title: "Confirm what is public",
          demo: "git log --oneline -3",
          check: "See that the bad commit is already on origin/main.",
          expect: "HEAD, main, and origin/main all point at the caching commit.",
          hint: "If origin/main contains the commit, do not rewrite it.",
          accept: [{
            re: /^git (log|status|branch)\b.*$/,
            run: function (s, input) { return observeLines(s, input); }
          }]
        },
        {
          title: "Revert the change",
          demo: "git revert HEAD --no-edit",
          check: "Create a new commit that undoes the bad patch.",
          expect: "A new tip appears; the caching commit remains in history.",
          hint: "git revert HEAD --no-edit skips the editor and uses the generated message.",
          blocks: [{
            re: /^git reset --hard/,
            message: "That rewrites a shared branch. Teammates who already pulled would have to recover. Use git revert."
          }, {
            re: /^git push --force/,
            message: "Force-push is not the fix for a published commit. Revert forward instead."
          }],
          accept: [{
            re: /^git revert HEAD( --no-edit)?$/,
            run: function (s) {
              var tip = findBySha(s, s.branches.main);
              var reverted = addCommit(s, 'Revert "Add response caching"', [tip.id], tip.lane);
              s.branches.main = reverted.id;
              s.reflog.unshift({ sha: reverted.id, action: 'revert: Revert "Add response caching"' });
              return [
                "[main " + short(reverted.id) + '] Revert "Add response caching"',
                " 3 files changed, 41 deletions(-)"
              ];
            }
          }]
        },
        {
          title: "Publish the revert",
          demo: "git push",
          check: "Fast-forward origin/main to the revert commit.",
          expect: "A normal push succeeds; no force required.",
          hint: "The revert only adds a commit, so the push is a fast-forward.",
          accept: [{
            re: /^git push( origin main)?$/,
            run: function (s) {
              var before = s.remote.main;
              s.remote.main = s.branches.main;
              return [
                "To github.com:acme/service.git",
                "   " + short(before) + ".." + short(s.branches.main) + "  main -> main"
              ];
            }
          }]
        },
        {
          title: "Read the resulting history",
          demo: "git log --oneline -3",
          check: "Confirm the revert is a commit, not a deletion.",
          expect: "The bad commit and its inverse both remain; the tip is the revert.",
          hint: "History is append-only on shared branches.",
          accept: [{
            re: /^git (log|status)\b.*$/,
            run: function (s, input) { return observeLines(s, input); }
          }]
        }
      ]
    },

    {
      id: "detached-head",
      title: "Work trapped in detached HEAD",
      tag: "DETACHED HEAD",
      difficulty: 2,
      brief: "You checked out an old commit and built a batch loader there. The warning said you were in detached HEAD; now main does not contain the work.",
      objectives: [
        "Recognize the detached state",
        "Create a branch at the detached commit",
        "Bring the commit onto main"
      ],
      note: {
        title: "Detached HEAD is a scratch checkout",
        body: "HEAD points directly at a commit instead of a branch. Commits are real, but no branch names them, so they disappear from view once you switch. Create a branch before switching away."
      },
      initial: function () {
        return {
          commits: [
            commit("e1aaaaa", "Initial commit", [], 0),
            commit("e2bbbbb", "Add catalog page", ["e1aaaaa"], 0),
            commit("e3ccccc", "Add search box", ["e2bbbbb"], 0),
            commit("e4ddddd", "Add pagination", ["e3ccccc"], 0),
            commit("e5eeeee", "Experiment: batch loader", ["e2bbbbb"], 1)
          ],
          branches: { main: "e4ddddd" },
          remote: { main: "e4ddddd" },
          head: { sha: "e5eeeee" },
          detachedFrom: "e2bbbbb",
          upstream: { main: "main" },
          files: { staged: [], modified: [], untracked: [] },
          rebase: null,
          conflict: null,
          origHead: "e5eeeee",
          reflog: [
            { sha: "e5eeeee", action: "commit: Experiment: batch loader" },
            { sha: "e3ccccc", action: "checkout: moving from main to e2bbbbb" },
            { sha: "e4ddddd", action: "commit: Add pagination" }
          ]
        };
      },
      steps: [
        {
          title: "Read the warning",
          demo: "git status",
          check: "Identify why main does not contain the experiment.",
          expect: "HEAD is detached from Add catalog page; no branch points at the new commit.",
          hint: "The status text changes completely when HEAD is detached.",
          accept: [{
            re: /^git (status|log|branch)\b.*$/,
            run: function (s, input) { return observeLines(s, input); }
          }]
        },
        {
          title: "Pin the commit with a branch",
          demo: "git switch -c rescue/experiment",
          check: "Create a branch at the detached commit before switching.",
          expect: "rescue/experiment now names the batch loader commit.",
          hint: "git switch -c <name> from detached HEAD anchors the commit.",
          blocks: [{
            re: /^git (switch|checkout) main$/,
            message: "That leaves the commit unnamed. Create the branch first, then switch."
          }],
          accept: [{
            re: /^git (switch -c|checkout -b) rescue\/experiment$/,
            run: function (s) {
              s.branches["rescue/experiment"] = headSha(s);
              s.head = { branch: "rescue/experiment" };
              return ["Switched to a new branch 'rescue/experiment'"];
            }
          }]
        },
        {
          title: "Return to main",
          demo: "git switch main",
          check: "Go back to the branch that should receive the work.",
          expect: "HEAD is on main; the rescue branch keeps the experiment safe.",
          hint: "Now that the commit is named, switching is safe.",
          accept: [{
            re: /^git (switch|checkout) main$/,
            run: function (s) {
              s.head = { branch: "main" };
              return ["Switched to branch 'main'"];
            }
          }]
        },
        {
          title: "Copy the commit onto main",
          demo: "git cherry-pick e5eeeee",
          check: "Replay the experiment on top of main.",
          expect: "A new commit with the same change sits on main; the rescue branch is unchanged.",
          hint: "git cherry-pick <sha> copies one commit. A merge is the alternative when you want the branch history.",
          accept: [{
            re: /^git cherry-pick ([0-9a-f]{7,40})$/,
            run: function (s, input) {
              var target = findBySha(s, input.split(" ").pop());
              if (!target) return ["fatal: bad revision"];
              var copied = addCommit(s, target.msg, [s.branches.main], 0);
              s.branches.main = copied.id;
              return [
                "[main " + short(copied.id) + "] " + target.msg,
                " 1 file changed, 87 insertions(+)"
              ];
            }
          }, {
            re: /^git merge rescue\/experiment$/,
            run: function (s) {
              var merged = addCommit(s, "Merge branch 'rescue/experiment'", [s.branches.main, s.branches["rescue/experiment"]], 0);
              s.branches.main = merged.id;
              return ["Merge made by the 'ort' strategy."];
            }
          }]
        },
        {
          title: "Verify both branches",
          demo: "git log --oneline --graph --decorate --all -5",
          check: "Confirm main contains the change and the rescue branch still exists.",
          expect: "Two refs, one copied change; nothing is dangling.",
          hint: "Use --all to see the rescue branch in the graph.",
          accept: [{
            re: /^git (log|branch|status)\b.*$/,
            run: function (s, input) { return observeLines(s, input); }
          }]
        }
      ]
    },

    {
      id: "reflog-rescue",
      title: "The rebase dropped my commits",
      tag: "REFLOG",
      difficulty: 3,
      brief: "An interactive rebase was supposed to squash two commits. It dropped them instead, and the branch now points at a rewritten tip. You have not pushed it.",
      objectives: [
        "Find the pre-rebase tip in the reflog",
        "Reset the branch back to it",
        "Verify the recovered history"
      ],
      note: {
        title: "Reflog is the safety net",
        body: "Every time HEAD moves, git records the old and new value in .git/logs/HEAD. git reflog shows those moves. A hard reset to the pre-rebase entry restores the branch; nothing was ever deleted from the object database."
      },
      initial: function () {
        return {
          commits: [
            commit("f1aaaaa", "Initial commit", [], 0),
            commit("f2bbbbb", "Add search", ["f1aaaaa"], 0),
            commit("f3ccccc", "Add tests", ["f2bbbbb"], 0),
            commit("f4ddddd", "Fix CI", ["f1aaaaa"], 2)
          ],
          branches: { main: "f4ddddd" },
          remote: { main: "f3ccccc" },
          head: { branch: "main" },
          upstream: { main: "main" },
          files: { staged: [], modified: [], untracked: [] },
          rebase: null,
          conflict: null,
          origHead: "f3ccccc",
          reflog: [
            { sha: "f4ddddd", action: "rebase (finish): returning to refs/heads/main" },
            { sha: "f4ddddd", action: "rebase (start): checkout f1aaaaa" },
            { sha: "f3ccccc", action: "commit: Add tests" },
            { sha: "f2bbbbb", action: "commit: Add search" }
          ]
        };
      },
      steps: [
        {
          title: "Read the reflog",
          demo: "git reflog",
          check: "Find the entry from before the rebase started.",
          expect: "HEAD@{2} is the Add tests commit; the rebase entries sit above it.",
          hint: "git reflog lists newest first, with the action that moved HEAD.",
          accept: [{
            re: /^git reflog\b.*$/,
            run: function (s) { return reflogLines(s); }
          }]
        },
        {
          title: "Restore the pre-rebase tip",
          demo: "git reset --hard HEAD@{2}",
          check: "Move main back to the commit that still contains both changes.",
          expect: "main points at Add tests; the rewritten tip becomes unreachable.",
          hint: "HEAD@{2} or ORIG_HEAD both name the old tip in this state.",
          blocks: [{
            re: /^git reset --hard HEAD@\{0\}$/,
            message: "HEAD@{0} is the broken rewritten tip. Move further back, to HEAD@{2}."
          }],
          accept: [{
            re: /^git reset --hard (HEAD@\{2\}|ORIG_HEAD)$/,
            run: function (s) {
              s.branches.main = s.origHead;
              s.reflog.unshift({ sha: s.origHead, action: "reset: moving to HEAD@{2}" });
              return ["HEAD is now at " + short(s.origHead) + " Add tests"];
            }
          }]
        },
        {
          title: "Verify the recovery",
          demo: "git log --oneline -4",
          check: "Confirm both the search and test commits are back.",
          expect: "The tip is Add tests with Add search beneath it.",
          hint: "The graph should show a straight line again.",
          accept: [{
            re: /^git (log|status|branch)\b.*$/,
            run: function (s, input) { return observeLines(s, input); }
          }]
        }
      ]
    }
  ];

  /* ------------------------------------------------------------------ */
  /* Engine                                                             */
  /* ------------------------------------------------------------------ */

  function initScenario(scenario) {
    var state = JSON.parse(JSON.stringify(scenario.initial()));
    state.seq = 100;
    state.stepIndex = 0;
    state.stars = MAX_STARS;
    state.mistakes = 0;
    state.hints = 0;
    state.history = [];
    state.scenarioId = scenario.id;
    return state;
  }

  function penalty(state) {
    state.stars = Math.max(1, state.stars - 1);
  }

  function execute(scenario, state, raw) {
    var input = String(raw || "").trim().replace(/\s+/g, " ");
    if (!input) return { status: "empty", lines: [] };
    if (!state.history) state.history = [];
    state.history.push(input);

    if (state.stepIndex >= scenario.steps.length) {
      return { status: "done", lines: [cmdLine(input), { kind: "warn", text: "Mission already complete. Restart it to replay." }], finished: true };
    }

    var step = scenario.steps[state.stepIndex];
    var i;

    for (i = 0; i < (step.blocks || []).length; i++) {
      if (step.blocks[i].re.test(input)) {
        state.mistakes += 1;
        penalty(state);
        return {
          status: "blocked",
          lines: [cmdLine(input), { kind: "warn", text: step.blocks[i].message }],
          shake: true
        };
      }
    }

    for (i = 0; i < (step.accept || []).length; i++) {
      if (step.accept[i].re.test(input)) {
        var produced = step.accept[i].run(state, input) || [];
        var lines = [cmdLine(input)].concat(outLines(produced));
        state.stepIndex += 1;
        lines.push({ kind: "step", text: "✓ " + step.title });
        var finished = state.stepIndex >= scenario.steps.length;
        if (!finished) {
          var next = scenario.steps[state.stepIndex];
          if (next.expect) lines.push({ kind: "expect", text: "Expect: " + next.expect });
          if (next.check) lines.push({ kind: "hint", text: "Next: " + next.check });
        }
        return { status: "ok", lines: lines, finished: finished };
      }
    }

    var observed = observeLines(state, input);
    if (observed) {
      return { status: "info", lines: [cmdLine(input)].concat(outLines(observed)) };
    }

    // A command that belongs to a later step is not wrong, just early.
    for (i = state.stepIndex + 1; i < scenario.steps.length; i++) {
      for (var j = 0; j < (scenario.steps[i].accept || []).length; j++) {
        if (scenario.steps[i].accept[j].re.test(input)) {
          return {
            status: "early",
            lines: [cmdLine(input), {
              kind: "warn",
              text: "Not yet. First finish: " + step.title + "."
            }],
            shake: true
          };
        }
      }
    }

    state.mistakes += 1;
    penalty(state);
    return {
      status: "error",
      lines: [cmdLine(input), {
        kind: "error",
        text: "That does not move the repository toward the objective. Check `git status`, then follow the mission step."
      }],
      shake: true
    };
  }

  function hintFor(scenario, state) {
    var step = scenario.steps[state.stepIndex];
    if (!step) return "Mission complete.";
    state.hints += 1;
    penalty(state);
    return step.hint || "Read the current state with git status.";
  }

  function scoreFor(state) {
    return Math.max(1, Math.min(MAX_STARS, state.stars));
  }

  /* ------------------------------------------------------------------ */
  /* Browser controller                                                  */
  /* ------------------------------------------------------------------ */

  function boot() {
    var lab = document.getElementById("gitlab");
    if (!lab) return;

    var els = {
      missions: document.getElementById("lab-missions"),
      title: document.getElementById("lab-title"),
      tag: document.getElementById("lab-tag"),
      brief: document.getElementById("lab-brief"),
      objectives: document.getElementById("lab-objectives"),
      difficulty: document.getElementById("lab-difficulty"),
      progress: document.getElementById("lab-progress"),
      score: document.getElementById("lab-stars"),
      graph: document.getElementById("lab-graph"),
      nodes: document.getElementById("lab-nodes"),
      edges: document.getElementById("lab-edges"),
      branch: document.getElementById("lab-branch"),
      head: document.getElementById("lab-head"),
      remote: document.getElementById("lab-remote"),
      state: document.getElementById("lab-state"),
      log: document.getElementById("lab-log"),
      form: document.getElementById("lab-form"),
      input: document.getElementById("lab-input"),
      hint: document.getElementById("lab-hint"),
      reveal: document.getElementById("lab-reveal"),
      restart: document.getElementById("lab-restart"),
      reset: document.getElementById("lab-reset"),
      noteTitle: document.getElementById("lab-note-title"),
      noteBody: document.getElementById("lab-note-body"),
      win: document.getElementById("lab-win"),
      winStars: document.getElementById("lab-win-stars"),
      winText: document.getElementById("lab-win-text"),
      winNext: document.getElementById("lab-win-next"),
      winClose: document.getElementById("lab-win-close")
    };

    var progress = loadProgress();
    var index = 0;
    for (var i = 0; i < SCENARIOS.length; i++) {
      if (!progress[SCENARIOS[i].id]) { index = i; break; }
    }
    var scenario = SCENARIOS[index];
    var state = initScenario(scenario);
    var rendered = {};
    var historyAt = -1;

    function loadProgress() {
      try {
        return JSON.parse(root.localStorage.getItem(STORAGE_KEY) || "{}");
      } catch (err) {
        return {};
      }
    }

    function saveProgress() {
      try {
        root.localStorage.setItem(STORAGE_KEY, JSON.stringify(progress));
      } catch (err) {
        /* private mode: the lab still works, it just will not remember. */
      }
    }

    function el(tag, className, text) {
      var node = document.createElement(tag);
      if (className) node.className = className;
      if (text !== undefined) node.textContent = text;
      return node;
    }

    function print(entry) {
      var line = el("p", "lab-line lab-line--" + (entry.kind || "out"), entry.text);
      els.log.appendChild(line);
      els.log.scrollTop = els.log.scrollHeight;
    }

    function resetLog() {
      els.log.textContent = "";
      print({ kind: "sys", text: "RECOVERY LAB // " + scenario.title.toUpperCase() });
      print({ kind: "sys", text: scenario.brief });
      print({ kind: "hint", text: "Type a git command, or press Show command for the next step." });
    }

    function renderMissions() {
      els.missions.textContent = "";
      SCENARIOS.forEach(function (mission, i) {
        var done = progress[mission.id];
        var item = el("li", "lab-mission" + (i === index ? " is-active" : "") + (done ? " is-done" : ""));
        var button = el("button", "lab-mission-btn");
        button.type = "button";
        button.setAttribute("aria-current", i === index ? "step" : "false");
        var num = el("span", "lab-mission-num", done ? "✓" : String(i + 1).padStart(2, "0"));
        var body = el("span", "lab-mission-body");
        body.appendChild(el("strong", null, mission.title));
        body.appendChild(el("small", null, mission.tag + " · " + "★".repeat(mission.difficulty)));
        button.appendChild(num);
        button.appendChild(body);
        button.addEventListener("click", function () {
          index = i;
          scenario = mission;
          state = initScenario(scenario);
          rendered = {};
          hideWin();
          resetLog();
          renderAll();
          els.input.focus();
        });
        item.appendChild(button);
        els.missions.appendChild(item);
      });
    }

    function renderBrief() {
      els.title.textContent = scenario.title;
      els.tag.textContent = scenario.tag;
      els.brief.textContent = scenario.brief;
      els.difficulty.textContent = "DIFFICULTY " + "★".repeat(scenario.difficulty) + "☆".repeat(Math.max(0, 3 - scenario.difficulty));
      els.progress.textContent = "STEP " + Math.min(state.stepIndex + 1, scenario.steps.length) + " / " + scenario.steps.length;
      els.score.textContent = "★".repeat(scoreFor(state)) + "☆".repeat(MAX_STARS - scoreFor(state));
      els.objectives.textContent = "";
      scenario.objectives.forEach(function (text) {
        els.objectives.appendChild(el("li", null, text));
      });
    }

    function renderFacts() {
      var branch = state.head.branch || "(detached)";
      els.branch.textContent = branch;
      els.head.textContent = short(headSha(state));
      var remotes = Object.keys(state.remote).map(function (name) {
        return "origin/" + name + " " + short(state.remote[name]);
      });
      els.remote.textContent = remotes.join(" · ") || "none";
      if (state.conflict) {
        els.state.textContent = "CONFLICT";
        els.state.className = "lab-state is-conflict";
      } else if (state.rebase) {
        els.state.textContent = "REBASING";
        els.state.className = "lab-state is-busy";
      } else if (!state.head.branch) {
        els.state.textContent = "DETACHED";
        els.state.className = "lab-state is-busy";
      } else {
        els.state.textContent = "CLEAN";
        els.state.className = "lab-state";
      }
    }

    function refLabel(s, sha) {
      var refs = refsFor(s, sha);
      if (!refs.length) return "";
      return refs.join(" · ");
    }

    function renderGraph() {
      var width = Math.max(520, 130 + state.commits.length * 132);
      var maxLane = 0;
      state.commits.forEach(function (c) { maxLane = Math.max(maxLane, c.lane || 0); });
      var height = 150 + maxLane * 104;
      els.graph.style.setProperty("--graph-w", width + "px");
      els.graph.style.setProperty("--graph-h", height + "px");
      els.nodes.textContent = "";
      els.edges.textContent = "";
      els.edges.setAttribute("viewBox", "0 0 " + width + " " + height);
      els.edges.setAttribute("width", width);
      els.edges.setAttribute("height", height);

      var positions = {};
      var live = reachableFromTips(state);
      var nextRendered = {};

      state.commits.forEach(function (c, i) {
        var x = 88 + i * 132;
        var y = 74 + (c.lane || 0) * 104;
        positions[c.id] = { x: x, y: y };
        var node = el("div", "lab-node");
        node.style.left = x + "px";
        node.style.top = y + "px";

        if (!live[c.id]) node.classList.add("is-orphan");
        if (!rendered[c.id]) node.classList.add("is-new");
        if (headSha(state) === c.id && state.conflict) node.classList.add("is-conflict");
        if (headSha(state) === c.id) node.classList.add("is-head");

        var dot = el("span", "lab-dot");
        dot.appendChild(el("span", "lab-sha", short(c.id)));
        node.appendChild(dot);
        node.appendChild(el("span", "lab-msg", c.msg));

        var refs = refsFor(state, c.id);
        if (refs.length) {
          var refRow = el("span", "lab-refs");
          refs.forEach(function (ref) {
            var cls = ref.indexOf("HEAD") === 0 ? "lab-ref is-head" : (ref.indexOf("origin/") === 0 ? "lab-ref is-remote" : "lab-ref");
            refRow.appendChild(el("span", cls, ref));
          });
          node.appendChild(refRow);
        }
        if (!live[c.id]) node.appendChild(el("span", "lab-orphan-tag", "unreachable"));
        els.nodes.appendChild(node);
        nextRendered[c.id] = true;
      });

      state.commits.forEach(function (c) {
        c.parents.forEach(function (parent) {
          if (!positions[parent] || !positions[c.id]) return;
          var from = positions[parent];
          var to = positions[c.id];
          var mid = (from.x + to.x) / 2;
          var path = document.createElementNS("http://www.w3.org/2000/svg", "path");
          var d = "M " + from.x + " " + from.y +
            " C " + mid + " " + from.y + ", " + mid + " " + to.y + ", " + to.x + " " + to.y;
          path.setAttribute("d", d);
          path.setAttribute("class", "lab-edge" + (live[c.id] ? "" : " is-orphan") + (rendered[c.id] ? "" : " is-new"));
          els.edges.appendChild(path);
        });
      });

      rendered = nextRendered;
    }

    function renderNote() {
      var step = scenario.steps[state.stepIndex];
      var note = (step && step.note) || scenario.note;
      els.noteTitle.textContent = note ? note.title : "";
      els.noteBody.textContent = note ? note.body : "";
    }

    function renderAll() {
      renderMissions();
      renderBrief();
      renderFacts();
      renderGraph();
      renderNote();
    }

    function run(raw) {
      var result = execute(scenario, state, raw);
      result.lines.forEach(print);
      if (result.shake) {
        els.graph.classList.remove("is-shaking");
        void els.graph.offsetWidth;
        els.graph.classList.add("is-shaking");
      }
      renderAll();
      if (result.finished) finish();
      return result;
    }

    function finish() {
      var stars = scoreFor(state);
      var previous = progress[scenario.id];
      if (!previous || stars > previous.stars) {
        progress[scenario.id] = { stars: stars, at: new Date().toISOString() };
        saveProgress();
      }
      renderMissions();
      els.winStars.textContent = "★".repeat(stars) + "☆".repeat(MAX_STARS - stars);
      els.winText.textContent = scenario.title + " — repaired with " + state.mistakes +
        " wrong move" + (state.mistakes === 1 ? "" : "s") + " and " + state.hints + " hint" + (state.hints === 1 ? "" : "s") + ".";
      els.win.hidden = false;
      var next = SCENARIOS[(SCENARIOS.indexOf(scenario) + 1) % SCENARIOS.length];
      els.winNext.textContent = "Next: " + next.title;
      els.winNext.onclick = function () {
        hideWin();
        var found = SCENARIOS.indexOf(scenario);
        index = (found + 1) % SCENARIOS.length;
        scenario = SCENARIOS[index];
        state = initScenario(scenario);
        rendered = {};
        resetLog();
        renderAll();
        els.input.focus();
      };
    }

    function hideWin() {
      els.win.hidden = true;
    }

    els.form.addEventListener("submit", function (event) {
      event.preventDefault();
      var raw = els.input.value;
      els.input.value = "";
      run(raw);
    });

    els.input.addEventListener("keydown", function (event) {
      if (event.key === "ArrowUp") {
        event.preventDefault();
        if (!state.history.length) return;
        historyAt = historyAt < 0 ? state.history.length - 1 : Math.max(0, historyAt - 1);
        els.input.value = state.history[historyAt];
      }
      if (event.key === "ArrowDown") {
        event.preventDefault();
        if (historyAt < 0) return;
        historyAt += 1;
        if (historyAt >= state.history.length) {
          historyAt = -1;
          els.input.value = "";
        } else {
          els.input.value = state.history[historyAt];
        }
      }
    });

    els.hint.addEventListener("click", function () {
      var text = hintFor(scenario, state);
      print({ kind: "hint", text: "HINT: " + text });
      renderBrief();
      renderAll();
    });

    els.reveal.addEventListener("click", function () {
      var step = scenario.steps[state.stepIndex];
      if (!step) return;
      print({ kind: "hint", text: "COMMAND: " + step.demo });
      state.hints += 1;
      penalty(state);
      els.input.value = step.demo;
      els.input.focus();
      renderBrief();
      renderAll();
    });

    els.restart.addEventListener("click", function () {
      state = initScenario(scenario);
      rendered = {};
      resetLog();
      renderAll();
      els.input.focus();
    });

    els.reset.addEventListener("click", function () {
      progress = {};
      saveProgress();
      index = 0;
      scenario = SCENARIOS[0];
      state = initScenario(scenario);
      rendered = {};
      resetLog();
      renderAll();
      els.input.focus();
    });

    els.winClose.addEventListener("click", hideWin);

    document.addEventListener("keydown", function (event) {
      if (event.key === "Escape" && !els.win.hidden) hideWin();
    });

    resetLog();
    renderAll();
    els.input.focus();
  }

  var api = {
    SCENARIOS: SCENARIOS,
    initScenario: initScenario,
    execute: execute,
    hintFor: hintFor,
    scoreFor: scoreFor,
    headSha: headSha,
    statusLines: statusLines,
    logLines: logLines
  };

  if (typeof module !== "undefined" && module.exports) {
    module.exports = api;
  } else {
    root.GitLab = api;
  }

  if (typeof document !== "undefined") {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", boot);
    } else {
      boot();
    }
  }
})(typeof window !== "undefined" ? window : globalThis);
