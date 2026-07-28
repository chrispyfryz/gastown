package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/steveyegge/gastown/internal/style"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// PatrolConfig holds role-specific patrol configuration.
type PatrolConfig struct {
	RoleName        string   // "deacon", "witness", "refinery"
	PatrolMolName   string   // "mol-deacon-patrol", etc.
	BeadsDir        string   // where to look for beads
	Assignee        string   // agent identity for pinning
	HeaderEmoji     string   // display emoji
	HeaderTitle     string   // "Patrol Status", etc.
	WorkLoopSteps   []string // role-specific instructions
	CheckInProgress bool     // whether to check in_progress status first (witness/refinery do, deacon doesn't)
}

// findActivePatrol finds an active patrol molecule for the role.
// Returns the patrol ID, display line, and whether one was found.
func findActivePatrol(cfg PatrolConfig) (patrolID, patrolLine string, found bool) {
	// Check for in-progress patrol first (if configured)
	if cfg.CheckInProgress {
		cmdList := exec.Command("bd", "--no-daemon", "list", "--status=in_progress", "--type=epic")
		cmdList.Dir = cfg.BeadsDir
		var stdoutList, stderrList bytes.Buffer
		cmdList.Stdout = &stdoutList
		cmdList.Stderr = &stderrList

		if err := cmdList.Run(); err != nil {
			if errMsg := strings.TrimSpace(stderrList.String()); errMsg != "" {
				fmt.Fprintf(os.Stderr, "bd list: %s\n", errMsg)
			}
		} else {
			lines := strings.Split(stdoutList.String(), "\n")
			for _, line := range lines {
				if strings.Contains(line, cfg.PatrolMolName) && !strings.Contains(line, "[template]") {
					parts := strings.Fields(line)
					if len(parts) > 0 {
						return parts[0], line, true
					}
				}
			}
		}
	}

	// Check for open patrols with open children (active wisp)
	cmdOpen := exec.Command("bd", "--no-daemon", "list", "--status=open", "--type=epic")
	cmdOpen.Dir = cfg.BeadsDir
	var stdoutOpen, stderrOpen bytes.Buffer
	cmdOpen.Stdout = &stdoutOpen
	cmdOpen.Stderr = &stderrOpen

	if err := cmdOpen.Run(); err != nil {
		if errMsg := strings.TrimSpace(stderrOpen.String()); errMsg != "" {
			fmt.Fprintf(os.Stderr, "bd list: %s\n", errMsg)
		}
	} else {
		lines := strings.Split(stdoutOpen.String(), "\n")
		for _, line := range lines {
			if strings.Contains(line, cfg.PatrolMolName) && !strings.Contains(line, "[template]") {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					molID := parts[0]
					// Check if this molecule has open children
					cmdShow := exec.Command("bd", "--no-daemon", "show", molID)
					cmdShow.Dir = cfg.BeadsDir
					var stdoutShow, stderrShow bytes.Buffer
					cmdShow.Stdout = &stdoutShow
					cmdShow.Stderr = &stderrShow
					if err := cmdShow.Run(); err != nil {
						if errMsg := strings.TrimSpace(stderrShow.String()); errMsg != "" {
							fmt.Fprintf(os.Stderr, "bd show: %s\n", errMsg)
						}
					} else {
						showOutput := stdoutShow.String()
						// Deacon only checks "- open]", witness/refinery also check "- in_progress]"
						hasOpenChildren := strings.Contains(showOutput, "- open]")
						if cfg.CheckInProgress {
							hasOpenChildren = hasOpenChildren || strings.Contains(showOutput, "- in_progress]")
						}
						if hasOpenChildren {
							return molID, line, true
						}
					}
				}
			}
		}
	}

	return "", "", false
}

// autoSpawnPatrol creates and pins a new patrol wisp.
// Returns the patrol ID or an error.
func autoSpawnPatrol(cfg PatrolConfig) (string, error) {
	// Create the patrol wisp directly from the patrol formula.
	// Note: the "molecule catalog" is a separate template mechanism and may not
	// include formula-defined protos like mol-refinery-patrol.
	cmdSpawn := exec.Command("bd", "--no-daemon", "mol", "wisp", cfg.PatrolMolName, "--actor", cfg.RoleName)
	cmdSpawn.Dir = cfg.BeadsDir
	var stdoutSpawn, stderrSpawn bytes.Buffer
	cmdSpawn.Stdout = &stdoutSpawn
	cmdSpawn.Stderr = &stderrSpawn

	if err := cmdSpawn.Run(); err != nil {
		errMsg := strings.TrimSpace(stderrSpawn.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("failed to create patrol wisp from %s: %s", cfg.PatrolMolName, errMsg)
	}

	// Parse the created molecule ID from output
	var patrolID string
	spawnOutput := stdoutSpawn.String()
	for _, line := range strings.Split(spawnOutput, "\n") {
		// Current bd output format:
		//   Root issue: mc-wisp-xyz
		// (Older versions used wisp-... or gt-... ids.)
		if strings.Contains(line, "Root issue:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				patrolID = strings.TrimSpace(parts[2])
				break
			}
		}
	}

	if patrolID == "" {
		return "", fmt.Errorf("created wisp but could not parse ID from output")
	}

	// Assign the wisp to the role so it is discoverable via `bd list` on next startup.
	// (Do not change status to "hooked" here; that status isn't part of bd's normal
	// list filters and will cause repeated re-spawns.)
	cmdPin := exec.Command("bd", "--no-daemon", "update", patrolID, "--assignee="+cfg.Assignee)
	cmdPin.Dir = cfg.BeadsDir
	if err := cmdPin.Run(); err != nil {
		return patrolID, fmt.Errorf("created wisp %s but failed to assign", patrolID)
	}

	return patrolID, nil
}

// outputPatrolContext is the main function that handles patrol display logic.
// It finds or creates a patrol and outputs the status and work loop.
func outputPatrolContext(cfg PatrolConfig) {
	fmt.Println()
	fmt.Printf("%s\n\n", style.Bold.Render(fmt.Sprintf("## %s %s", cfg.HeaderEmoji, cfg.HeaderTitle)))

	// Try to find an active patrol
	patrolID, patrolLine, hasPatrol := findActivePatrol(cfg)

	if !hasPatrol {
		// No active patrol - auto-spawn one
		fmt.Printf("Status: **No active patrol** - creating %s...\n", cfg.PatrolMolName)
		fmt.Println()

		var err error
		patrolID, err = autoSpawnPatrol(cfg)
		if err != nil {
			if patrolID != "" {
				fmt.Printf("⚠ %s\n", err.Error())
			} else {
				fmt.Println(style.Dim.Render(err.Error()))
				fmt.Println(style.Dim.Render("Troubleshooting:"))
				fmt.Println(style.Dim.Render("  - `bd formula list | rg patrol`"))
				fmt.Println(style.Dim.Render("  - `bd mol seed --patrol`"))
				return
			}
		} else {
			fmt.Printf("✓ Created and assigned patrol wisp: %s\n", patrolID)
		}
	} else {
		// Has active patrol - show status
		fmt.Println("Status: **Patrol Active**")
		fmt.Printf("Patrol: %s\n\n", strings.TrimSpace(patrolLine))
	}

	// Show patrol work loop instructions
	fmt.Printf("**%s Patrol Work Loop:**\n", cases.Title(language.English).String(cfg.RoleName))
	for i, step := range cfg.WorkLoopSteps {
		fmt.Printf("%d. %s\n", i+1, step)
	}

	if patrolID != "" {
		fmt.Println()
		fmt.Printf("Current patrol ID: %s\n", patrolID)
	}
}
