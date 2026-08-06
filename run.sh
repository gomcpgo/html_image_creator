#!/bin/bash
set -e

# HTML Image Creator MCP Server Build/Test Script

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

command=$1
shift || true

case "$command" in
    build)
        echo "Building html_image_creator..."
        mkdir -p bin
        go build -o bin/html_image_creator cmd/main.go
        echo "Build complete: bin/html_image_creator"
        ;;

    test)
        echo "Running tests..."
        go test ./... -v
        ;;

    install)
        echo "Installing dependencies..."
        go mod download
        go mod tidy
        ;;

    create)
        if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ]; then
            echo "Usage: ./run.sh create <name> <html_content> <width> <height>"
            exit 1
        fi
        bin/html_image_creator -create "$1" -html "$2" -width "$3" -height "$4"
        ;;

    list)
        bin/html_image_creator -list
        ;;

    get)
        if [ -z "$1" ]; then
            echo "Usage: ./run.sh get <post_id>"
            exit 1
        fi
        bin/html_image_creator -get "$1"
        ;;

    update)
        if [ -z "$1" ] || [ -z "$2" ]; then
            echo "Usage: ./run.sh update <post_id> <html_content>"
            exit 1
        fi
        bin/html_image_creator -update "$1" -html "$2"
        ;;

    export)
        if [ -z "$1" ] || [ -z "$2" ]; then
            echo "Usage: ./run.sh export <post_id> <output_path>"
            exit 1
        fi
        bin/html_image_creator -export "$1" -output "$2"
        ;;

    add-media)
        if [ -z "$1" ] || [ -z "$2" ]; then
            echo "Usage: ./run.sh add-media <post_id> <media_path>"
            exit 1
        fi
        bin/html_image_creator -add-media "$1" -media-path "$2"
        ;;

    render-frames)
        if [ -z "$1" ]; then
            echo "Usage: ./run.sh render-frames <spec_file> [--validate-only]"
            exit 1
        fi
        if [ "$2" == "--validate-only" ]; then
            bin/html_image_creator -render-frames "$1" -validate-only
        else
            bin/html_image_creator -render-frames "$1"
        fi
        ;;

    create-chart)
        if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ]; then
            echo "Usage: ./run.sh create-chart <name> <html_content> <width> <height> <data> [data_format]"
            exit 1
        fi
        FORMAT="${6:-json}"
        bin/html_image_creator -create-chart "$1" -html "$2" -width "$3" -height "$4" -data "$5" -data-format "$FORMAT"
        ;;

    set-data)
        if [ -z "$1" ] || [ -z "$2" ]; then
            echo "Usage: ./run.sh set-data <post_id> <data> [data_format]"
            exit 1
        fi
        FORMAT="${3:-json}"
        bin/html_image_creator -set-data "$1" -data "$2" -data-format "$FORMAT"
        ;;

    test-chart)
        echo "=== Chart Visualisation Smoke Test ==="
        echo ""

        # Build first
        echo "Building..."
        mkdir -p bin
        go build -o bin/html_image_creator cmd/main.go
        echo ""

        # Chart HTML with Chart.js bar chart for quarterly SaaS revenue
        CHART_HTML='<!DOCTYPE html>
<html><head>
<script src="libs/chart.min.js"></script>
<style>body{margin:0;padding:20px;background:#1a1a2e;font-family:Arial,sans-serif;}</style>
</head><body>
<canvas id="chart"></canvas>
<script>
fetch("data.json").then(r=>r.json()).then(data=>{
  new Chart(document.getElementById("chart"),{
    type:"bar",
    data:{
      labels:data.map(d=>d.quarter),
      datasets:[
        {label:"Revenue ($)",data:data.map(d=>d.revenue),backgroundColor:"rgba(99,102,241,0.8)",yAxisID:"y"},
        {label:"New Customers",data:data.map(d=>d.customers),backgroundColor:"rgba(16,185,129,0.8)",yAxisID:"y1"}
      ]
    },
    options:{
      responsive:true,
      plugins:{title:{display:true,text:"SaaS Quarterly Performance 2025",color:"#e0e0e0",font:{size:18}}},
      scales:{
        x:{ticks:{color:"#e0e0e0"},grid:{color:"#333"}},
        y:{position:"left",title:{display:true,text:"Revenue ($)",color:"#e0e0e0"},ticks:{color:"#e0e0e0"},grid:{color:"#333"}},
        y1:{position:"right",title:{display:true,text:"New Customers",color:"#e0e0e0"},ticks:{color:"#e0e0e0"},grid:{display:false}}
      }
    }
  });
  document.title="ready";
});
</script>
</body></html>'

        CHART_DATA='[{"quarter":"Q1 2025","revenue":142000,"customers":85},{"quarter":"Q2 2025","revenue":168000,"customers":112},{"quarter":"Q3 2025","revenue":195000,"customers":134},{"quarter":"Q4 2025","revenue":231000,"customers":158}]'

        echo "Step 1: Creating chart..."
        CREATE_OUTPUT=$(bin/html_image_creator -create-chart "SaaS Quarterly Report" -html "$CHART_HTML" -width 800 -height 500 -data "$CHART_DATA" -data-format json)
        echo "$CREATE_OUTPUT"
        POST_ID=$(echo "$CREATE_OUTPUT" | python3 -c "import sys,json; print(json.load(sys.stdin)['post_id'])")
        echo "Post ID: $POST_ID"
        echo ""

        echo "Step 2: Exporting chart to PNG..."
        OUTPUT_PATH="/tmp/saas-quarterly-chart.png"
        bin/html_image_creator -export "$POST_ID" -output "$OUTPUT_PATH"
        echo ""

        if [ -f "$OUTPUT_PATH" ]; then
            SIZE=$(wc -c < "$OUTPUT_PATH" | tr -d ' ')
            echo "SUCCESS: Chart exported to $OUTPUT_PATH ($SIZE bytes)"
            echo ""

            echo "Step 3: Updating data with Q1 2026 projection..."
            UPDATED_DATA='[{"quarter":"Q1 2025","revenue":142000,"customers":85},{"quarter":"Q2 2025","revenue":168000,"customers":112},{"quarter":"Q3 2025","revenue":195000,"customers":134},{"quarter":"Q4 2025","revenue":231000,"customers":158},{"quarter":"Q1 2026","revenue":270000,"customers":185}]'
            bin/html_image_creator -set-data "$POST_ID" -data "$UPDATED_DATA" -data-format json
            echo ""

            echo "Step 4: Re-exporting with updated data..."
            UPDATED_PATH="/tmp/saas-quarterly-chart-updated.png"
            bin/html_image_creator -export "$POST_ID" -output "$UPDATED_PATH"
            echo ""

            if [ -f "$UPDATED_PATH" ]; then
                USIZE=$(wc -c < "$UPDATED_PATH" | tr -d ' ')
                echo "SUCCESS: Updated chart exported to $UPDATED_PATH ($USIZE bytes)"
            else
                echo "FAIL: Updated chart export failed"
                exit 1
            fi

            # Open the images on macOS
            if command -v open &> /dev/null; then
                echo ""
                echo "Opening images..."
                open "$OUTPUT_PATH"
                open "$UPDATED_PATH"
            fi
        else
            echo "FAIL: Chart export failed - no file at $OUTPUT_PATH"
            exit 1
        fi

        echo ""
        echo "=== Smoke test complete ==="
        ;;

    clean)
        echo "Cleaning build artifacts..."
        rm -rf bin
        echo "Clean complete"
        ;;

    *)
        echo "HTML Image Creator MCP Server"
        echo ""
        echo "Usage: ./run.sh <command> [args]"
        echo ""
        echo "Commands:"
        echo "  build                                           Build the MCP server"
        echo "  test                                            Run tests"
        echo "  install                                         Install dependencies"
        echo "  create <name> <html> <width> <height>           Create a new image post"
        echo "  list                                            List all image posts"
        echo "  get <id>                                        Get image post by ID"
        echo "  update <id> <html>                              Update image post content"
        echo "  export <id> <output_path>                       Export as PNG image"
        echo "  add-media <id> <path>                           Add media file to post"
        echo "  create-chart <name> <html> <w> <h> <data> [fmt] Create a chart post"
        echo "  set-data <id> <data> [format]                   Update chart data"
        echo "  test-chart                                      Run chart smoke test"
        echo "  clean                                           Remove build artifacts"
        echo ""
        echo "Examples:"
        echo "  ./run.sh build"
        echo "  ./run.sh create 'My Post' '<div style=\"background:red\">Hello</div>' 1080 1080"
        echo "  ./run.sh list"
        echo "  ./run.sh export my-post-a3f9 /tmp/output.png"
        echo "  ./run.sh test-chart"
        ;;
esac
