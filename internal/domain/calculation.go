package domain

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func BuildPlan(id, caseID string, revision int, circuits []Circuit, capacity float64, submitted time.Time) (ElectricalPlan, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(caseID) == "" {
		return ElectricalPlan{}, invalid("id", "方案标识不能为空")
	}
	if revision < 1 {
		return ElectricalPlan{}, invalid("revision", "版本号必须大于零")
	}
	if len(circuits) == 0 {
		return ElectricalPlan{}, invalid("circuits", "至少登记一个回路")
	}
	if capacity <= 0 {
		return ElectricalPlan{}, invalid("designCapacityKVA", "设计容量必须大于零")
	}
	seen := map[string]bool{}
	checks := make([]ProtectionCheck, 0, len(circuits))
	total := 0.0
	allPassed := true
	for i, circuit := range circuits {
		if circuit.ID == "" {
			return ElectricalPlan{}, invalid(fmt.Sprintf("circuits[%d].id", i), "不能为空")
		}
		if seen[circuit.ID] {
			return ElectricalPlan{}, invalid("circuits", "回路编号不能重复")
		}
		seen[circuit.ID] = true
		check, err := CheckCircuit(circuit)
		if err != nil {
			return ElectricalPlan{}, err
		}
		checks = append(checks, check)
		total += circuit.PowerKW
		if !check.BreakerAdequate || !check.CableAdequate || !check.RCDCompliant {
			allPassed = false
		}
	}
	result := "PASS"
	if total/0.9 > capacity || !allPassed {
		result = "FAIL"
	}
	return ElectricalPlan{ID: id, CaseID: caseID, Revision: revision, Circuits: append([]Circuit(nil), circuits...), TotalLoadKW: round(total, 2), DesignCapacityKVA: capacity, ProtectionChecks: checks, CalculationResult: result, SubmittedAt: submitted}, nil
}

func CheckCircuit(c Circuit) (ProtectionCheck, error) {
	if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Equipment) == "" {
		return ProtectionCheck{}, invalid("circuit", "回路名称和设备不能为空")
	}
	if c.PowerKW <= 0 || c.VoltageV <= 0 {
		return ProtectionCheck{}, invalid("circuit", "功率和电压必须大于零")
	}
	if c.Phases != 1 && c.Phases != 3 {
		return ProtectionCheck{}, invalid("phases", "相数只能为 1 或 3")
	}
	current := c.PowerKW * 1000 / c.VoltageV
	if c.Phases == 3 {
		current = c.PowerKW * 1000 / (math.Sqrt(3) * c.VoltageV * 0.9)
	}
	breakerOK := c.BreakerA >= current*1.1 && c.BreakerA <= current*2.5
	ampacity := cableAmpacity(c.CableMM2, c.Phases)
	cableOK := ampacity >= c.BreakerA
	rcdOK := c.RCDMilliA > 0 && c.RCDMilliA <= 30
	messages := []string{}
	if !breakerOK {
		messages = append(messages, "断路器额定电流与计算电流不匹配")
	}
	if !cableOK {
		messages = append(messages, "电缆载流量低于断路器额定电流")
	}
	if !rcdOK {
		messages = append(messages, "末端回路剩余电流保护值应不大于 30mA")
	}
	if len(messages) == 0 {
		messages = append(messages, "保护参数符合核算边界")
	}
	return ProtectionCheck{CircuitID: c.ID, CalculatedCurrentA: round(current, 2), BreakerAdequate: breakerOK, CableAdequate: cableOK, RCDCompliant: rcdOK, Messages: messages}, nil
}

func cableAmpacity(area float64, phases int) float64 {
	base := map[float64]float64{1.5: 18, 2.5: 25, 4: 33, 6: 42, 10: 58, 16: 76, 25: 101, 35: 125, 50: 151}
	value := base[area]
	if phases == 3 {
		value *= 0.87
	}
	return value
}

func round(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}
