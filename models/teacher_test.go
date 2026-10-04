package models

import "testing"

func TestTeacherDisplayName(t *testing.T) {
	cases := []struct{ full, nick, gender, want string }{
		{"তুষার আহমেদ", "তুষার", "male", "তুষার স্যার"},
		{"আয়েশা খাতুন", "আয়েশা", "female", "আয়েশা ম্যাম"},
		{"Riyad Hasan", "", "male", "Riyad Hasan স্যার"},
		{"Riyad Hasan", "রিয়াজ", "", "রিয়াজ"},
		{"", "", "", ""},
	}
	for _, c := range cases {
		if got := TeacherDisplayName(c.full, c.nick, c.gender); got != c.want {
			t.Errorf("TeacherDisplayName(%q,%q,%q) = %q, want %q", c.full, c.nick, c.gender, got, c.want)
		}
	}
}

func TestTeacherDisplayNameEn(t *testing.T) {
	if got := TeacherDisplayNameEn("Tushar Ahmed", "Tushar", "male"); got != "Tushar Sir" {
		t.Errorf("got %q", got)
	}
	if got := TeacherDisplayNameEn("Ayesha Khatun", "", "female"); got != "Ayesha Khatun Ma'am" {
		t.Errorf("got %q", got)
	}
	if got := TeacherDisplayNameEn("Ayesha Khatun", "Ayesha", ""); got != "Ayesha" {
		t.Errorf("got %q", got)
	}
}
