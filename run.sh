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
        echo "  build                                  Build the MCP server"
        echo "  test                                   Run tests"
        echo "  install                                Install dependencies"
        echo "  create <name> <html> <width> <height>  Create a new image post"
        echo "  list                                   List all image posts"
        echo "  get <id>                               Get image post by ID"
        echo "  update <id> <html>                     Update image post content"
        echo "  export <id> <output_path>              Export as PNG image"
        echo "  add-media <id> <path>                  Add media file to post"
        echo "  clean                                  Remove build artifacts"
        echo ""
        echo "Examples:"
        echo "  ./run.sh build"
        echo "  ./run.sh create 'My Post' '<div style=\"background:red\">Hello</div>' 1080 1080"
        echo "  ./run.sh list"
        echo "  ./run.sh export my-post-a3f9 /tmp/output.png"
        ;;
esac
