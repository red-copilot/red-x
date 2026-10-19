package reporting

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Aabyss-Team/ARL/internal/domain"
	"github.com/Aabyss-Team/ARL/internal/store"
	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Exporter struct {
	Store *store.Store
	Dir   string
}

func (e *Exporter) ExportTask(ctx context.Context, taskID bson.ObjectID, format string) (string, error) {
	task, err := e.Store.GetTask(ctx, taskID)
	if err != nil || task == nil {
		return "", fmt.Errorf("task not found")
	}
	obs, err := e.allObservations(ctx, taskID)
	if err != nil {
		return "", err
	}
	findings, err := e.allFindings(ctx, taskID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(e.Dir, 0o750); err != nil {
		return "", err
	}
	name := fmt.Sprintf("ARL_%s_%d.%s", task.ID.Hex(), time.Now().Unix(), format)
	path := filepath.Join(e.Dir, name)
	switch format {
	case "csv":
		if err := writeCSV(path, obs, findings); err != nil {
			return "", err
		}
	case "xlsx":
		if err := writeXLSX(path, task, obs, findings); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported format %q", format)
	}
	return path, nil
}

func (e *Exporter) allObservations(ctx context.Context, taskID bson.ObjectID) ([]domain.AssetObservation, error) {
	var out []domain.AssetObservation
	for page := 1; ; page++ {
		items, total, err := e.Store.ListTaskObservations(ctx, taskID, "", store.PageQuery{Page: page, PageSize: 200})
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(items) == 0 || int64(page*200) >= total {
			break
		}
	}
	return out, nil
}

func (e *Exporter) allFindings(ctx context.Context, taskID bson.ObjectID) ([]domain.Finding, error) {
	var out []domain.Finding
	for page := 1; ; page++ {
		items, total, err := e.Store.ListFindings(ctx, "", taskID.Hex(), "", store.PageQuery{Page: page, PageSize: 200})
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(items) == 0 || int64(page*200) >= total {
			break
		}
	}
	return out, nil
}

func writeCSV(path string, obs []domain.AssetObservation, findings []domain.Finding) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"type", "key", "source", "observedAt", "data"})
	for _, o := range obs {
		_ = w.Write([]string{string(o.Kind), o.Key, o.Source, o.ObservedAt.Format(time.RFC3339), fmt.Sprint(o.Snapshot)})
	}
	for _, x := range findings {
		_ = w.Write([]string{string(x.Kind), x.Target, x.PluginName, x.CreatedAt.Format(time.RFC3339), x.Title})
	}
	return w.Error()
}

func writeXLSX(path string, task *domain.Task, obs []domain.AssetObservation, findings []domain.Finding) error {
	f := excelize.NewFile()
	defer f.Close()
	summary := f.GetSheetName(0)
	_ = f.SetSheetName(summary, "Summary")
	rows := [][]any{
		{"ID", task.ID.Hex()}, {"Name", task.Name}, {"Target", task.Target}, {"Status", task.Status},
		{"Domains", task.Statistics.Domains}, {"IPs", task.Statistics.IPs}, {"Sites", task.Statistics.Sites},
		{"Vulns", task.Statistics.Vulns}, {"Nuclei", task.Statistics.Nuclei}, {"WIH", task.Statistics.WIH},
	}
	for i, row := range rows {
		for j, v := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+1)
			_ = f.SetCellValue("Summary", cell, v)
		}
	}
	_, _ = f.NewSheet("Assets")
	_ = f.SetSheetRow("Assets", "A1", &[]any{"Kind", "Key", "Source", "ObservedAt", "Data"})
	for i, o := range obs {
		row := []any{string(o.Kind), o.Key, o.Source, o.ObservedAt.Format(time.RFC3339), fmt.Sprint(o.Snapshot)}
		cell := "A" + strconv.Itoa(i+2)
		_ = f.SetSheetRow("Assets", cell, &row)
	}
	_, _ = f.NewSheet("Findings")
	_ = f.SetSheetRow("Findings", "A1", &[]any{"Kind", "Title", "Target", "Plugin", "Severity", "CreatedAt"})
	for i, x := range findings {
		row := []any{string(x.Kind), x.Title, x.Target, x.PluginName, x.Severity, x.CreatedAt.Format(time.RFC3339)}
		cell := "A" + strconv.Itoa(i+2)
		_ = f.SetSheetRow("Findings", cell, &row)
	}
	return f.SaveAs(path)
}
