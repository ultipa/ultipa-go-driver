package test

import (
	"testing"
)

func TestShowAlgo(t *testing.T) {
	//client, _ := GetClient(hosts, graph)

	algos, err := client.ShowHDCAlgo("hdc-server-1", nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(algos) == 0 {
		t.Log("no algo return")
	}
	//printers.PrintAlgoList(algos)
}

var (
	algoName = "lpa"
	hdcName  = "hdc-server-1"
)

func TestAlgo(t *testing.T) {
	//algo, _ := client.GetAlgo(algoName, nil)
	//
	//UninstallHDCAlgo
	//removeErr := fmt.Sprintf(`remove .//algo/libs//libplugin_%s.so failed!`, algoName)
	//_, err := client.UninstallHDCAlgo(algoName, hdcName, nil)
	//if algo == nil && !strings.Contains(err.Error(), removeErr) {
	//	t.Errorf("UninstallHDCAlgo failed %v", err)
	//}
	//
	//if algo != nil && err != nil {
	//	t.Errorf("UninstallHDCAlgo failed %v", err)
	//}
	//
	//// UninstallHDCAlgo not exist again
	//_, err = client.UninstallHDCAlgo(algoName, hdcName, nil)
	//if !strings.Contains(err.Error(), removeErr) {
	//	t.Errorf("Uninstall not exist Algo failed %v", err)
	//}
	//
	//// UninstallHDCAlgo empty algoName, will success
	//_, err = client.UninstallHDCAlgo("", hdcName, nil)
	//if err != nil {
	//	t.Errorf("UninstallHDCAlgo empty algoName failed %v", err)
	//}

	// InstallHDCAlgo
	_, err := client.InstallHDCAlgo([]string{"./test_algo_lib/libplugin_lpa.so", "./test_algo_lib/lpa.yml"}, hdcName, nil)
	//files := map[string]string{
	//	"./test_algo_lib/libplugin_lpa.so": "./test_algo_lib/lpa.yml",
	//	//"./test_algo_lib/libplugin_lpa.so": "./test_algo_lib/lpa.yml",
	//}
	//_, err := client.InstallHDCAlgo(files, hdcName, nil)

	if err != nil {
		t.Errorf("InstallHDCAlgo error, %v", err)
	}

	//_, err = client.InstallHDCAlgo("./test_algo_lib/libplugin_lpa.so", "./test_algo_lib/lpa.yml", hdcName, nil)
	//versionErr := fmt.Sprintf("libplugin_%s.so:The new algo version must be greater than old!", algoName)
	//if !strings.Contains(err.Error(), versionErr) {
	//    t.Errorf("InstallHDCAlgo error, %v", err)
	//}

	//algo, _ := client.GetAlgo(algoName, nil)
	//if algo == nil {
	//    t.Error("No installed algorithm found")
	//}

	// UninstallHDCAlgo Avoid unexpected problems due to inconsistent algorithm versions and servers
	if !t.Run("UninstallHDCAlgo", TestUninstallAlgo) {
		t.Error("UninstallHDCAlgo failed")
	}
}

func TestUninstallAlgo(t *testing.T) {
	_, err := client.UninstallHDCAlgo(algoName, hdcName, nil)

	if err != nil {
		t.Fatalf("UninstallHDCAlgo error, %v", err)
	}
}

func TestRollbackHDCAlgo(t *testing.T) {
	_, err := client.RollbackHDCAlgo(algoName, hdcName, nil)

	if err != nil {
		t.Fatalf("RollbackHDCAlgo error, %v", err)
	}
}

// TestInstallHDCAlgoOnlySo tests installing algo with only .so file (no yml)
func TestInstallHDCAlgoOnlySo(t *testing.T) {
	// Test: Install with only .so file should succeed
	_, err := client.InstallHDCAlgo([]string{"./test_algo_lib/libplugin_lpa.so"}, hdcName, nil)
	if err != nil {
		t.Errorf("InstallHDCAlgo with only .so file should succeed, but got error: %v", err)
	}

	// Cleanup: uninstall the algo
	_, _ = client.UninstallHDCAlgo(algoName, hdcName, nil)
}

// TestInstallHDCAlgoMultipleSo tests installing algo with multiple .so files
func TestInstallHDCAlgoMultipleSo(t *testing.T) {
	// Test: Install with multiple .so files should succeed
	_, err := client.InstallHDCAlgo([]string{
		"./test_algo_lib/libplugin_lpa.so",
		"./test_algo_lib/libplugin_k_core.so",
	}, hdcName, nil)
	if err != nil {
		t.Errorf("InstallHDCAlgo with multiple .so files should succeed, but got error: %v", err)
	}

	// Cleanup: uninstall the algos
	_, _ = client.UninstallHDCAlgo(algoName, hdcName, nil)
	_, _ = client.UninstallHDCAlgo("k_core", hdcName, nil)
}

// TestInstallHDCAlgoValidation tests parameter validation
func TestInstallHDCAlgoValidation(t *testing.T) {
	// Test: Empty files should return error
	_, err := client.InstallHDCAlgo([]string{}, hdcName, nil)
	if err == nil || err.Error() != "empty files" {
		t.Errorf("InstallHDCAlgo with empty files should return 'empty files' error, got: %v", err)
	}

	// Test: Nil files should return error
	_, err = client.InstallHDCAlgo(nil, hdcName, nil)
	if err == nil || err.Error() != "empty files" {
		t.Errorf("InstallHDCAlgo with nil files should return 'empty files' error, got: %v", err)
	}

	// Test: Only yml file (no .so) should return error
	_, err = client.InstallHDCAlgo([]string{"./test_algo_lib/lpa.yml"}, hdcName, nil)
	if err == nil || err.Error() != "at least one .so file is required" {
		t.Errorf("InstallHDCAlgo with only yml file should return 'at least one .so file is required' error, got: %v", err)
	}
}
