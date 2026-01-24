package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateWorktree(t *testing.T) {
	// Create a temporary git repo for testing
	tmpDir, err := os.MkdirTemp("", "coders-worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	if err := exec.Command("git", "-C", tmpDir, "init").Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "add", ".").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Test worktree creation
	sessionName := "test-session"
	worktreePath, err := createWorktree(tmpDir, sessionName)
	if err != nil {
		t.Fatalf("createWorktree failed: %v", err)
	}

	// Verify worktree path (resolve symlinks for comparison on macOS)
	expectedPath := filepath.Join(tmpDir, ".coders", "worktrees", sessionName)
	expectedPathResolved, _ := filepath.EvalSymlinks(expectedPath)
	worktreePathResolved, _ := filepath.EvalSymlinks(worktreePath)
	if worktreePathResolved != expectedPathResolved {
		t.Errorf("Expected worktree path %s, got %s", expectedPathResolved, worktreePathResolved)
	}

	// Verify worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("Worktree directory does not exist: %s", worktreePath)
	}

	// Verify branch was created
	branchName := "session/" + sessionName
	checkBranchCmd := exec.Command("git", "-C", tmpDir, "rev-parse", "--verify", branchName)
	if err := checkBranchCmd.Run(); err != nil {
		t.Errorf("Branch %s was not created", branchName)
	}

	// Verify worktree is on the correct branch
	getCurrentBranchCmd := exec.Command("git", "-C", worktreePath, "branch", "--show-current")
	output, err := getCurrentBranchCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get current branch: %v", err)
	}
	currentBranch := strings.TrimSpace(string(output))
	if currentBranch != branchName {
		t.Errorf("Expected branch %s, got %s", branchName, currentBranch)
	}

	// Verify worktree appears in git worktree list
	listCmd := exec.Command("git", "-C", tmpDir, "worktree", "list")
	listOutput, err := listCmd.Output()
	if err != nil {
		t.Fatalf("Failed to list worktrees: %v", err)
	}
	if !strings.Contains(string(listOutput), worktreePath) {
		t.Errorf("Worktree %s not found in git worktree list", worktreePath)
	}

	// Test duplicate worktree creation fails
	_, err = createWorktree(tmpDir, sessionName)
	if err == nil {
		t.Error("Expected error when creating duplicate worktree, got nil")
	}
	if !strings.Contains(err.Error(), "worktree already exists") {
		t.Errorf("Expected 'worktree already exists' error, got: %v", err)
	}
}

func TestFindGitRoot(t *testing.T) {
	// Create a temporary git repo
	tmpDir, err := os.MkdirTemp("", "coders-gitroot-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	if err := exec.Command("git", "-C", tmpDir, "init").Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Test finding git root from repo root
	gitRoot, err := findGitRoot(tmpDir)
	if err != nil {
		t.Fatalf("findGitRoot failed: %v", err)
	}
	// Resolve symlinks for comparison on macOS
	tmpDirResolved, _ := filepath.EvalSymlinks(tmpDir)
	gitRootResolved, _ := filepath.EvalSymlinks(gitRoot)
	if gitRootResolved != tmpDirResolved {
		t.Errorf("Expected git root %s, got %s", tmpDirResolved, gitRootResolved)
	}

	// Create a subdirectory and test from there
	subDir := filepath.Join(tmpDir, "subdir", "nested")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	gitRoot, err = findGitRoot(subDir)
	if err != nil {
		t.Fatalf("findGitRoot from subdir failed: %v", err)
	}
	gitRootResolved, _ = filepath.EvalSymlinks(gitRoot)
	if gitRootResolved != tmpDirResolved {
		t.Errorf("Expected git root %s from subdir, got %s", tmpDirResolved, gitRootResolved)
	}

	// Test non-git directory
	nonGitDir, err := os.MkdirTemp("", "coders-nongit-test-*")
	if err != nil {
		t.Fatalf("Failed to create non-git temp dir: %v", err)
	}
	defer os.RemoveAll(nonGitDir)

	_, err = findGitRoot(nonGitDir)
	if err == nil {
		t.Error("Expected error for non-git directory, got nil")
	}
}
