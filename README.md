# TuiEditor

Go로 작성된 터미널(CLI) 환경용 텍스트 에디터입니다.  
VSCode와 유사한 UI 구조(파일 트리 + 탭 + 에디터 + 상태바)를 완전히 터미널 안에서 구현합니다.

## 스크린샷

```
┌── FileTree ──┬── [file1.go] [file2.go ●] ──── [Save] [Close] [Exit] ─┐
│              │                                                         │
│  src/        │  package main                                           │
│  ├── main.go │                                                         │
│  └── ...     │  import (                                               │
│              │      "fmt"                                              │
│              │  )                                                       │
└──────────────┴─────────────────────────────────────────────────────────┘
│  { } [ ] ( ) < > / \ | & ; : = ! ? # @ " ' ` ~ $ % ^ * + - _         │
│  NORMAL  /path/to/file.go                     Ln 10, Col 5  42 lines  │
└─────────────────────────────────────────────────────────────────────────┘
```

## 주요 기능

| 기능 | 설명 |
|---|---|
| **멀티탭 에디터** | 여러 파일을 탭으로 동시에 편집, 탭 전환 시 상태 완전 보존 |
| **파일 트리** | 디렉토리 계층 탐색, 폴더 접기/펼치기, 상위 디렉토리 이동 |
| **신택스 하이라이팅** | 파일 확장자 자동 감지, Monokai 테마, 100+ 언어 지원 |
| **Word-Wrap** | 긴 줄 자동 줄바꿈, 커서 이동도 wrap 경계 인식 |
| **마우스 지원** | 클릭으로 커서 이동, 탭/버튼 클릭, 파일 트리 조작 |
| **특수문자 툴바** | 30개 특수문자 버튼 클릭으로 즉시 삽입 |
| **스크롤바** | 에디터 우측에 시각적 스크롤바(`▓`/`░`) 표시 |
| **동적 레이아웃** | 터미널 리사이즈 시 모든 컴포넌트 크기 자동 재계산 |

## 단축키

### 전역

| 단축키 | 동작 |
|---|---|
| `Ctrl+Q` | 앱 종료 |
| `Ctrl+T` | 파일 트리로 포커스 이동 |
| `Ctrl+E` | 에디터로 포커스 이동 |
| `Alt+]` | 다음 탭 |
| `Alt+[` | 이전 탭 |
| `Ctrl+W` | 현재 탭 닫기 |
| `Ctrl+S` | 파일 저장 |

### 에디터

| 단축키 | 동작 |
|---|---|
| `↑` `↓` `←` `→` | 커서 이동 |
| `Home` / `Ctrl+A` | 줄 처음으로 |
| `End` / `Ctrl+E` | 줄 끝으로 |
| `PageUp` / `Ctrl+B` | 페이지 위 |
| `PageDown` / `Ctrl+F` | 페이지 아래 |
| `Alt+←` / `Ctrl+←` | 단어 단위 왼쪽 이동 |
| `Alt+→` / `Ctrl+→` | 단어 단위 오른쪽 이동 |
| `Enter` | 새 줄 삽입 |
| `Backspace` | 뒤 문자 삭제 |
| `Delete` / `Ctrl+D` | 앞 문자 삭제 |
| `Tab` | 탭 문자 삽입 |

### 파일 트리

| 단축키 | 동작 |
|---|---|
| `↑` `↓` | 항목 이동 |
| `Enter` / 더블클릭 | 파일 열기 / 폴더 접기·펼치기 |
| `-` / `Backspace` | 상위 디렉토리로 이동 |

## 프로젝트 구조

```
TuiEditor/
├── main.go                    # 앱 진입점 & 최상위 모델
└── internal/
    ├── editor/editor.go       # 텍스트 에디터 핵심 로직
    ├── filetree/filetree.go   # 파일 트리 탐색기
    ├── tabs/tabs.go           # 탭 바
    ├── statusbar/statusbar.go # 상태바
    └── toolbar/toolbar.go     # 특수문자 삽입 툴바
```

## 기술 스택

| 구분 | 라이브러리 | 버전 |
|---|---|---|
| TUI 프레임워크 | [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) | v1.3.10 |
| 스타일링 | [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) | v1.1.0 |
| TUI 컴포넌트 | [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles) | v1.0.0 |
| 신택스 하이라이팅 | [alecthomas/chroma](https://github.com/alecthomas/chroma) | v2.23.1 |
| 유니코드 처리 | [mattn/go-runewidth](https://github.com/mattn/go-runewidth), [rivo/uniseg](https://github.com/rivo/uniseg) | - |

아키텍처는 BubbleTea의 **Elm 아키텍처(Model-Update-View)** 패턴을 기반으로 하며,
각 UI 컴포넌트가 독립적인 `Model/Update/View` 구조를 가집니다.

## 빌드 및 실행

### 요구 사항

- Go 1.21 이상

### 로컬 빌드 & 실행

```sh
git clone https://github.com/your-repo/TuiEditor.git
cd TuiEditor
go build -o tuieditor .
./tuieditor
```

### 크로스 컴파일

모든 의존성이 순수 Go 라이브러리이므로 CGO 없이 다양한 플랫폼으로 크로스 컴파일이 가능합니다.

```sh
# Linux amd64 (서버/데스크탑 일반)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tuieditor-linux-amd64 .

# Linux arm64 (AWS Graviton, 라즈베리파이 64bit 등)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o tuieditor-linux-arm64 .

# Linux arm (라즈베리파이 32bit 등)
GOOS=linux GOARCH=arm CGO_ENABLED=0 go build -o tuieditor-linux-arm .

# Windows amd64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o tuieditor-windows-amd64.exe .
```

빌드 결과물은 외부 라이브러리 의존이 없는 **단일 실행 바이너리**입니다.

## 라이선스

MIT
