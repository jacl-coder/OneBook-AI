package app

import "testing"

func TestBuildBookDocumentProfileInfersInternshipCertificate(t *testing.T) {
	profile := buildBookDocumentProfile("赖新鹏实习证明.pdf", []chunkPayload{
		{
			Content: "实习证明\n兹证明 兰州理工大学 学生 赖新鹏 在我单位工程研发部门进行实习工作。\n特此证明！",
			Metadata: map[string]string{
				"source_type": "pdf",
				"source_ref":  "page:1",
				"page":        "1",
			},
		},
	})

	if profile.DocumentType != "internship_certificate" {
		t.Fatalf("DocumentType = %q, want internship_certificate", profile.DocumentType)
	}
	if profile.DocumentSummary == "" {
		t.Fatal("DocumentSummary is empty")
	}
	if profile.FirstPageText == "" {
		t.Fatal("FirstPageText is empty")
	}
	if len(profile.Keywords) == 0 {
		t.Fatal("Keywords is empty")
	}
}
