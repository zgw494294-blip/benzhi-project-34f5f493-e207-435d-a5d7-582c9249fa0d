package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"powerpermit/internal/application"
	"powerpermit/internal/domain"
)

func runSelfcheck(server *http.Server, address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("启动自检监听: %w", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	baseURL := "http://" + address
	client := &http.Client{Timeout: 2 * time.Second}
	if err := waitHealthy(ctx, client, baseURL); err != nil {
		return err
	}
	now := time.Now().UTC()
	create := application.CreateCaseCommand{Meta: application.Meta{Actor: "自检申请人", IdempotencyKey: "selfcheck-create", ExpectedVersion: 0}, ActivityName: "自检临时演出", Venue: "自检场地", StartAt: now.Add(time.Hour), EndAt: now.Add(8 * time.Hour), Contact: domain.Contact{Name: "张工", Phone: "13800000000"}, RiskLevel: domain.RiskMedium}
	created := application.CommandResponse{}
	if err := post(client, baseURL+"/api/cases", create, &created); err != nil {
		return err
	}
	caseID, version := created.Case.Case.ID, created.Case.Case.Version
	plan := application.SubmitPlanCommand{Meta: application.Meta{Actor: "自检方案工程师", IdempotencyKey: "selfcheck-plan", ExpectedVersion: version}, DesignCapacityKVA: 50, Circuits: []domain.Circuit{{ID: "C1", Name: "主回路", Equipment: "舞台灯光", PowerKW: 8, VoltageV: 380, Phases: 3, BreakerA: 20, RCDMilliA: 30, CableMM2: 4}}}
	planned := application.CommandResponse{}
	if err := post(client, baseURL+"/api/cases/"+caseID+"/plans", plan, &planned); err != nil {
		return err
	}
	version = planned.Case.Case.Version
	started := application.CommandResponse{}
	if err := post(client, baseURL+"/api/cases/"+caseID+"/inspections/start", application.StartInspectionCommand{Meta: application.Meta{Actor: "自检检查员", IdempotencyKey: "selfcheck-start", ExpectedVersion: version}}, &started); err != nil {
		return err
	}
	version = started.Case.Case.Version
	inspection := application.RecordInspectionCommand{Meta: application.Meta{Actor: "自检检查员", IdempotencyKey: "selfcheck-inspection", ExpectedVersion: version}, Items: []application.InspectionItem{
		{ItemCode: "GROUNDING", MeasuredValue: "接地电阻 2.1Ω", PhotoNote: "配电箱接地端子照片", Result: domain.FindingPass},
		{ItemCode: "RCD_TEST", MeasuredValue: "动作电流 26mA", PhotoNote: "测试仪读数照片", Result: domain.FindingPass},
		{ItemCode: "CABLE_ROUTE", MeasuredValue: "架空敷设并设跨越保护", Result: domain.FindingPass},
		{ItemCode: "PANEL_LOCK", MeasuredValue: "箱门上锁且标识完整", Result: domain.FindingPass},
		{ItemCode: "EMERGENCY", MeasuredValue: "应急断电按钮动作正常", Result: domain.FindingPass},
	}}
	inspected := application.CommandResponse{}
	if err := post(client, baseURL+"/api/cases/"+caseID+"/inspections", inspection, &inspected); err != nil {
		return err
	}
	version = inspected.Case.Case.Version
	reviewed := application.CommandResponse{}
	if err := post(client, baseURL+"/api/cases/"+caseID+"/review", application.ReviewCommand{Meta: application.Meta{Actor: "自检审批负责人", IdempotencyKey: "selfcheck-review", ExpectedVersion: version}, Passed: true}, &reviewed); err != nil {
		return err
	}
	version = reviewed.Case.Case.Version
	issued := application.CommandResponse{}
	if err := post(client, baseURL+"/api/cases/"+caseID+"/permit", application.IssueCommand{Meta: application.Meta{Actor: "自检审批负责人", IdempotencyKey: "selfcheck-issue", ExpectedVersion: version}}, &issued); err != nil {
		return err
	}
	if issued.Case.Permit == nil || issued.Case.Case.Status != domain.StatusPermitted {
		return errors.New("自检未生成送电许可")
	}
	var permit struct {
		Verified bool `json:"verified"`
	}
	if err := get(client, baseURL+"/api/cases/"+caseID+"/permit", &permit); err != nil {
		return err
	}
	if !permit.Verified {
		return errors.New("许可摘要校验失败")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if serveErr := <-serveDone; !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	return nil
}

func waitHealthy(ctx context.Context, client *http.Client, baseURL string) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		var health map[string]string
		if get(client, baseURL+"/healthz", &health) == nil && health["status"] == "ok" {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("等待 HTTP 自检服务超时")
		case <-ticker.C:
		}
	}
}

func post(client *http.Client, url string, input, output any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return err
	}
	response, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	return decodeResponse(response, output)
}

func get(client *http.Client, url string, output any) error {
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	return decodeResponse(response, output)
}

func decodeResponse(response *http.Response, output any) error {
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, body)
	}
	return json.NewDecoder(response.Body).Decode(output)
}
