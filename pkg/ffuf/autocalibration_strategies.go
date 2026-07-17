package ffuf

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AutocalibrationStrategy map[string][]string

func setupDefaultAutocalibrationStrategies() error {
	basic_strategy := AutocalibrationStrategy{
		"basic_admin":  []string{"admin" + RandomString(16), "admin" + RandomString(8)},
		"htaccess":     []string{".htaccess" + RandomString(16), ".htaccess" + RandomString(8)},
		"basic_random": []string{RandomString(16), RandomString(8)},
	}
	basic_strategy_json, err := json.Marshal(basic_strategy)
	if err != nil {
		return err
	}

	advanced_strategy := AutocalibrationStrategy{
		"basic_admin":  []string{"admin" + RandomString(16), "admin" + RandomString(8)},
		"htaccess":     []string{".htaccess" + RandomString(16), ".htaccess" + RandomString(8)},
		"basic_random": []string{RandomString(16), RandomString(8)},
		"admin_dir":    []string{"admin" + RandomString(16) + "/", "admin" + RandomString(8) + "/"},
		"random_dir":   []string{RandomString(16) + "/", RandomString(8) + "/"},
	}
	advanced_strategy_json, err := json.Marshal(advanced_strategy)
	if err != nil {
		return err
	}

	basic_strategy_file := filepath.Join(AUTOCALIBDIR, "basic.json")
	if !FileExists(basic_strategy_file) {
		err = os.WriteFile(filepath.Join(AUTOCALIBDIR, "basic.json"), basic_strategy_json, 0640)
		return err
	}
	advanced_strategy_file := filepath.Join(AUTOCALIBDIR, "advanced.json")
	if !FileExists(advanced_strategy_file) {
		err = os.WriteFile(filepath.Join(AUTOCALIBDIR, "advanced.json"), advanced_strategy_json, 0640)
		return err
	}

	return nil
}
