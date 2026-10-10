package replylang

import "testing"

// Spanish with few of the most common words still reads as Spanish, not a
// tie with Portuguese: a topic check's refusal in the quality run (#510).
func TestDetectSpanishNotPortuguese(t *testing.T) {
	if got, ok := Detect("Por favor, pregúnteme sobre neumáticos para temporadas frías o si necesita reservar una rotación de neumáticos."); !ok || got != "es" {
		t.Errorf("Detect = %q, %v; want es", got, ok)
	}
	if got, ok := Detect("Você pode me dizer qual é o melhor pneu para o inverno?"); !ok || got != "pt" {
		t.Errorf("Portuguese: Detect = %q, %v; want pt", got, ok)
	}
}
