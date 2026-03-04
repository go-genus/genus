package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/GabrielOnRails/genus/core"
)

// MigrationNode representa um nó no DAG de migrações.
type MigrationNode struct {
	Version     int64    `json:"version"`
	Description string   `json:"description"`
	Status      string   `json:"status"` // applied, pending, failed
	DependsOn   []int64  `json:"depends_on"`
	FileName    string   `json:"file_name"`
	CreatedAt   string   `json:"created_at,omitempty"`
	AppliedAt   string   `json:"applied_at,omitempty"`
	Duration    string   `json:"duration,omitempty"`
}

// MigrationDAG representa o grafo de migrações.
type MigrationDAG struct {
	Nodes []MigrationNode `json:"nodes"`
	Edges []MigrationEdge `json:"edges"`
}

// MigrationEdge representa uma aresta no DAG.
type MigrationEdge struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// MigrationVisualizer visualiza migrações como DAG.
type MigrationVisualizer struct {
	migrationsPath string
	executor       core.Executor
	appliedSet     map[int64]bool
}

// NewMigrationVisualizer cria um novo visualizador.
func NewMigrationVisualizer(migrationsPath string, executor core.Executor) *MigrationVisualizer {
	return &MigrationVisualizer{
		migrationsPath: migrationsPath,
		executor:       executor,
		appliedSet:     make(map[int64]bool),
	}
}

// BuildDAG constrói o DAG de migrações.
func (v *MigrationVisualizer) BuildDAG(ctx context.Context) (*MigrationDAG, error) {
	// Carrega migrações aplicadas
	if v.executor != nil {
		if err := v.loadAppliedMigrations(ctx); err != nil {
			// Ignora erro se tabela não existe
			fmt.Printf("Warning: could not load applied migrations: %v\n", err)
		}
	}

	// Lê arquivos de migração
	migrations, err := v.readMigrationFiles()
	if err != nil {
		return nil, err
	}

	// Constrói DAG
	dag := &MigrationDAG{
		Nodes: make([]MigrationNode, 0, len(migrations)),
		Edges: make([]MigrationEdge, 0),
	}

	for _, m := range migrations {
		status := "pending"
		if v.appliedSet[m.Version] {
			status = "applied"
		}

		node := MigrationNode{
			Version:     m.Version,
			Description: m.Description,
			Status:      status,
			FileName:    m.FileName,
			DependsOn:   m.DependsOn,
		}

		dag.Nodes = append(dag.Nodes, node)

		// Cria arestas para dependências
		for _, dep := range m.DependsOn {
			dag.Edges = append(dag.Edges, MigrationEdge{
				From: dep,
				To:   m.Version,
			})
		}
	}

	// Se não há dependências explícitas, assume sequência linear
	if len(dag.Edges) == 0 {
		sort.Slice(dag.Nodes, func(i, j int) bool {
			return dag.Nodes[i].Version < dag.Nodes[j].Version
		})

		for i := 1; i < len(dag.Nodes); i++ {
			dag.Edges = append(dag.Edges, MigrationEdge{
				From: dag.Nodes[i-1].Version,
				To:   dag.Nodes[i].Version,
			})
		}
	}

	return dag, nil
}

// MigrationInfo informações de uma migração.
type MigrationInfo struct {
	Version     int64
	Description string
	FileName    string
	DependsOn   []int64
}

func (v *MigrationVisualizer) loadAppliedMigrations(ctx context.Context) error {
	rows, err := v.executor.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return err
		}
		v.appliedSet[version] = true
	}

	return nil
}

func (v *MigrationVisualizer) readMigrationFiles() ([]MigrationInfo, error) {
	var migrations []MigrationInfo

	// Lê arquivos .go de migração
	entries, err := os.ReadDir(v.migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Pattern para extrair versão e descrição do nome do arquivo
	// Ex: 001_create_users.go -> version=1, description=create_users
	pattern := regexp.MustCompile(`^(\d+)_(.+)\.go$`)

	// Pattern para extrair dependências do código
	dependsPattern := regexp.MustCompile(`DependsOn:\s*\[\]int64\{([^}]+)\}`)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		matches := pattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}

		version := parseInt64(matches[1])
		description := strings.ReplaceAll(matches[2], "_", " ")

		// Lê o arquivo para encontrar dependências
		var dependsOn []int64
		content, err := os.ReadFile(filepath.Join(v.migrationsPath, entry.Name()))
		if err == nil {
			if depMatches := dependsPattern.FindStringSubmatch(string(content)); depMatches != nil {
				deps := strings.Split(depMatches[1], ",")
				for _, d := range deps {
					d = strings.TrimSpace(d)
					if d != "" {
						dependsOn = append(dependsOn, parseInt64(d))
					}
				}
			}
		}

		migrations = append(migrations, MigrationInfo{
			Version:     version,
			Description: description,
			FileName:    entry.Name(),
			DependsOn:   dependsOn,
		})
	}

	// Ordena por versão
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func parseInt64(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}

// OutputFormat formato de saída.
type OutputFormat string

const (
	OutputFormatJSON     OutputFormat = "json"
	OutputFormatHTML     OutputFormat = "html"
	OutputFormatDOT      OutputFormat = "dot"
	OutputFormatMermaid  OutputFormat = "mermaid"
	OutputFormatASCII    OutputFormat = "ascii"
)

// Visualize gera a visualização no formato especificado.
func (v *MigrationVisualizer) Visualize(ctx context.Context, format OutputFormat, w io.Writer) error {
	dag, err := v.BuildDAG(ctx)
	if err != nil {
		return err
	}

	switch format {
	case OutputFormatJSON:
		return v.outputJSON(dag, w)
	case OutputFormatHTML:
		return v.outputHTML(dag, w)
	case OutputFormatDOT:
		return v.outputDOT(dag, w)
	case OutputFormatMermaid:
		return v.outputMermaid(dag, w)
	case OutputFormatASCII:
		return v.outputASCII(dag, w)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func (v *MigrationVisualizer) outputJSON(dag *MigrationDAG, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(dag)
}

func (v *MigrationVisualizer) outputDOT(dag *MigrationDAG, w io.Writer) error {
	fmt.Fprintln(w, "digraph migrations {")
	fmt.Fprintln(w, "  rankdir=TB;")
	fmt.Fprintln(w, "  node [shape=box, style=rounded];")
	fmt.Fprintln(w)

	// Nodes
	for _, node := range dag.Nodes {
		color := "lightblue"
		if node.Status == "applied" {
			color = "lightgreen"
		} else if node.Status == "failed" {
			color = "lightcoral"
		}

		fmt.Fprintf(w, "  m%d [label=\"%d: %s\", fillcolor=%s, style=\"rounded,filled\"];\n",
			node.Version, node.Version, node.Description, color)
	}

	fmt.Fprintln(w)

	// Edges
	for _, edge := range dag.Edges {
		fmt.Fprintf(w, "  m%d -> m%d;\n", edge.From, edge.To)
	}

	fmt.Fprintln(w, "}")
	return nil
}

func (v *MigrationVisualizer) outputMermaid(dag *MigrationDAG, w io.Writer) error {
	fmt.Fprintln(w, "```mermaid")
	fmt.Fprintln(w, "graph TD")
	fmt.Fprintln(w)

	// Nodes
	for _, node := range dag.Nodes {
		shape := "[%s]"
		if node.Status == "applied" {
			shape = "([%s])"
		}

		label := fmt.Sprintf("%d: %s", node.Version, node.Description)
		fmt.Fprintf(w, "    M%d"+shape+"\n", node.Version, label)
	}

	fmt.Fprintln(w)

	// Edges
	for _, edge := range dag.Edges {
		fmt.Fprintf(w, "    M%d --> M%d\n", edge.From, edge.To)
	}

	fmt.Fprintln(w)

	// Styling
	fmt.Fprintln(w, "    classDef applied fill:#90EE90,stroke:#333")
	fmt.Fprintln(w, "    classDef pending fill:#87CEEB,stroke:#333")
	fmt.Fprintln(w, "    classDef failed fill:#F08080,stroke:#333")

	for _, node := range dag.Nodes {
		fmt.Fprintf(w, "    class M%d %s\n", node.Version, node.Status)
	}

	fmt.Fprintln(w, "```")
	return nil
}

func (v *MigrationVisualizer) outputASCII(dag *MigrationDAG, w io.Writer) error {
	fmt.Fprintln(w, "Migration DAG")
	fmt.Fprintln(w, "=============")
	fmt.Fprintln(w)

	// Agrupa por "camadas" (profundidade no DAG)
	depths := make(map[int64]int)
	v.calculateDepths(dag, depths)

	maxDepth := 0
	for _, d := range depths {
		if d > maxDepth {
			maxDepth = d
		}
	}

	// Imprime por camada
	for depth := 0; depth <= maxDepth; depth++ {
		var nodesAtDepth []MigrationNode
		for _, node := range dag.Nodes {
			if depths[node.Version] == depth {
				nodesAtDepth = append(nodesAtDepth, node)
			}
		}

		if len(nodesAtDepth) == 0 {
			continue
		}

		// Imprime nós nesta camada
		for _, node := range nodesAtDepth {
			status := "[PENDING]"
			if node.Status == "applied" {
				status = "[APPLIED]"
			} else if node.Status == "failed" {
				status = "[FAILED] "
			}

			fmt.Fprintf(w, "%s %03d: %s\n", status, node.Version, node.Description)
		}

		// Imprime setas para próxima camada
		if depth < maxDepth {
			hasArrow := false
			for _, edge := range dag.Edges {
				if depths[edge.From] == depth {
					hasArrow = true
					break
				}
			}
			if hasArrow {
				fmt.Fprintln(w, "    │")
				fmt.Fprintln(w, "    ▼")
			}
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Legend: [APPLIED] = executed, [PENDING] = not yet executed")

	return nil
}

func (v *MigrationVisualizer) calculateDepths(dag *MigrationDAG, depths map[int64]int) {
	// Inicializa com profundidade 0
	for _, node := range dag.Nodes {
		depths[node.Version] = 0
	}

	// Calcula profundidade baseado em dependências
	changed := true
	for changed {
		changed = false
		for _, edge := range dag.Edges {
			newDepth := depths[edge.From] + 1
			if newDepth > depths[edge.To] {
				depths[edge.To] = newDepth
				changed = true
			}
		}
	}
}

func (v *MigrationVisualizer) outputHTML(dag *MigrationDAG, w io.Writer) error {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Migration DAG Visualizer</title>
    <script src="https://d3js.org/d3.v7.min.js"></script>
    <script src="https://unpkg.com/dagre-d3@0.6.4/dist/dagre-d3.min.js"></script>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            margin: 0;
            padding: 20px;
            background: #1e1e1e;
            color: #d4d4d4;
        }
        h1 {
            color: #569cd6;
            margin-bottom: 20px;
        }
        .container {
            display: flex;
            gap: 20px;
        }
        .graph-container {
            flex: 2;
            background: #252526;
            border-radius: 8px;
            padding: 20px;
            min-height: 500px;
        }
        .list-container {
            flex: 1;
            background: #252526;
            border-radius: 8px;
            padding: 20px;
        }
        .migration-item {
            display: flex;
            align-items: center;
            padding: 10px;
            margin-bottom: 8px;
            background: #2d2d2d;
            border-radius: 4px;
            border-left: 4px solid #569cd6;
        }
        .migration-item.applied {
            border-left-color: #4ec9b0;
        }
        .migration-item.pending {
            border-left-color: #dcdcaa;
        }
        .migration-item.failed {
            border-left-color: #f14c4c;
        }
        .status-badge {
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 12px;
            margin-right: 10px;
        }
        .status-badge.applied {
            background: #4ec9b0;
            color: #1e1e1e;
        }
        .status-badge.pending {
            background: #dcdcaa;
            color: #1e1e1e;
        }
        .version {
            font-weight: bold;
            margin-right: 10px;
            color: #9cdcfe;
        }
        .description {
            color: #d4d4d4;
        }
        svg {
            width: 100%;
            height: 500px;
        }
        .node rect {
            stroke: #333;
            stroke-width: 2px;
        }
        .node.applied rect {
            fill: #4ec9b0;
        }
        .node.pending rect {
            fill: #dcdcaa;
        }
        .node.failed rect {
            fill: #f14c4c;
        }
        .edgePath path {
            stroke: #666;
            stroke-width: 2px;
            fill: none;
        }
        .edgePath marker {
            fill: #666;
        }
        text {
            fill: #1e1e1e;
            font-weight: 500;
        }
        .legend {
            margin-top: 20px;
            display: flex;
            gap: 20px;
        }
        .legend-item {
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .legend-color {
            width: 20px;
            height: 20px;
            border-radius: 4px;
        }
    </style>
</head>
<body>
    <h1>📊 Migration DAG Visualizer</h1>

    <div class="container">
        <div class="graph-container">
            <h2>Dependency Graph</h2>
            <svg id="graph"></svg>
        </div>

        <div class="list-container">
            <h2>Migrations</h2>
            {{range .Nodes}}
            <div class="migration-item {{.Status}}">
                <span class="status-badge {{.Status}}">{{.Status}}</span>
                <span class="version">#{{.Version}}</span>
                <span class="description">{{.Description}}</span>
            </div>
            {{end}}
        </div>
    </div>

    <div class="legend">
        <div class="legend-item">
            <div class="legend-color" style="background: #4ec9b0"></div>
            <span>Applied</span>
        </div>
        <div class="legend-item">
            <div class="legend-color" style="background: #dcdcaa"></div>
            <span>Pending</span>
        </div>
        <div class="legend-item">
            <div class="legend-color" style="background: #f14c4c"></div>
            <span>Failed</span>
        </div>
    </div>

    <script>
        const data = {{.}};

        // Create the graph
        var g = new dagreD3.graphlib.Graph()
            .setGraph({
                rankdir: 'TB',
                nodesep: 50,
                ranksep: 50,
                marginx: 20,
                marginy: 20
            })
            .setDefaultEdgeLabel(function() { return {}; });

        // Add nodes
        data.nodes.forEach(function(node) {
            g.setNode('m' + node.version, {
                label: node.version + ': ' + node.description,
                class: node.status,
                rx: 5,
                ry: 5,
                padding: 10
            });
        });

        // Add edges
        data.edges.forEach(function(edge) {
            g.setEdge('m' + edge.from, 'm' + edge.to, {
                arrowhead: 'vee'
            });
        });

        // Render
        var svg = d3.select('#graph');
        var inner = svg.append('g');

        var zoom = d3.zoom().on('zoom', function(event) {
            inner.attr('transform', event.transform);
        });
        svg.call(zoom);

        var render = new dagreD3.render();
        render(inner, g);

        // Center the graph
        var initialScale = 0.9;
        var graphWidth = g.graph().width * initialScale;
        var graphHeight = g.graph().height * initialScale;
        var svgWidth = parseInt(svg.style('width'));
        var svgHeight = parseInt(svg.style('height'));

        svg.call(zoom.transform, d3.zoomIdentity
            .translate((svgWidth - graphWidth) / 2, 20)
            .scale(initialScale));
    </script>
</body>
</html>`

	t, err := template.New("dag").Parse(tmpl)
	if err != nil {
		return err
	}

	return t.Execute(w, dag)
}

// ValidateDAG valida que o DAG não tem ciclos.
func (v *MigrationVisualizer) ValidateDAG(dag *MigrationDAG) error {
	// Detecta ciclos usando DFS
	visited := make(map[int64]bool)
	recStack := make(map[int64]bool)

	// Cria mapa de adjacência
	adj := make(map[int64][]int64)
	for _, edge := range dag.Edges {
		adj[edge.From] = append(adj[edge.From], edge.To)
	}

	var hasCycle func(node int64) bool
	hasCycle = func(node int64) bool {
		visited[node] = true
		recStack[node] = true

		for _, neighbor := range adj[node] {
			if !visited[neighbor] {
				if hasCycle(neighbor) {
					return true
				}
			} else if recStack[neighbor] {
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for _, node := range dag.Nodes {
		if !visited[node.Version] {
			if hasCycle(node.Version) {
				return fmt.Errorf("cycle detected in migration dependencies")
			}
		}
	}

	return nil
}

// GetExecutionOrder retorna a ordem de execução das migrações.
func (v *MigrationVisualizer) GetExecutionOrder(dag *MigrationDAG) ([]int64, error) {
	// Validação primeiro
	if err := v.ValidateDAG(dag); err != nil {
		return nil, err
	}

	// Ordenação topológica
	inDegree := make(map[int64]int)
	for _, node := range dag.Nodes {
		inDegree[node.Version] = 0
	}
	for _, edge := range dag.Edges {
		inDegree[edge.To]++
	}

	// Adjacência
	adj := make(map[int64][]int64)
	for _, edge := range dag.Edges {
		adj[edge.From] = append(adj[edge.From], edge.To)
	}

	// Kahn's algorithm
	var queue []int64
	for version, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, version)
		}
	}

	var order []int64
	for len(queue) > 0 {
		// Pega o menor (para ordem determinística)
		sort.Slice(queue, func(i, j int) bool {
			return queue[i] < queue[j]
		})

		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		for _, neighbor := range adj[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(order) != len(dag.Nodes) {
		return nil, fmt.Errorf("cycle detected - could not complete topological sort")
	}

	return order, nil
}
