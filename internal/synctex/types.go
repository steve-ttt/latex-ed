package synctex

// ForwardQuery specifies position in a LaTeX source file to find in PDF.
type ForwardQuery struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column,omitempty"`
	PdfFile string `json:"pdf_file"`
	Dir     string `json:"dir,omitempty"`
}

// ForwardResult contains the mapped location in the PDF.
type ForwardResult struct {
	Page   int     `json:"page"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// InverseQuery specifies a click location on a PDF page to find in source.
type InverseQuery struct {
	Page    int     `json:"page"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	PdfFile string  `json:"pdf_file"`
	Dir     string  `json:"dir,omitempty"`
}

// InverseResult contains the mapped file and line number in source.
type InverseResult struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}
