package insights

import "testing"

func TestTokens(t *testing.T) {
	toks := Tokens("Двойной ввод данных в 1С и Битрикс24, ещё раз! 2026")
	for _, want := range []string{"двойн", "ввод", "данны", "1с", "битри"} {
		if _, ok := toks[want]; !ok {
			t.Errorf("missing token %q in %v", want, toks)
		}
	}
	for _, bad := range []string{"в", "и", "ещё", "еще", "раз", "2026"} {
		if _, ok := toks[bad]; ok {
			t.Errorf("stop-word/number %q must be dropped", bad)
		}
	}
	// stemming makes word forms match
	if Jaccard(Tokens("поставки оборудования"), Tokens("поставка оборудованию")) != 1 {
		t.Error("5-char prefix stemming should unify word forms")
	}
}

func TestJaccardAndScore(t *testing.T) {
	if Jaccard(map[string]struct{}{}, map[string]struct{}{"a": {}}) != 0 {
		t.Error("empty set")
	}
	a := Doc{ID: 1, CategoryID: 1, Title: "Задержка поставки камер", Body: "Поставщик задерживает поставку камер видеонаблюдения, срываются сроки монтажа"}
	b := Doc{ID: 2, CategoryID: 1, Title: "Задержки поставки оборудования", Body: "Оборудование от поставщика приходит с задержкой, сроки монтажа срываются"}
	c := Doc{ID: 3, CategoryID: 2, Title: "Кофемашина не работает", Body: "На третьем этаже не работает кофемашина вторую неделю"}
	sameCat := Score(&a, &b)
	b2 := b
	b2.CategoryID = 9
	b2.tokens = nil
	if Score(&a, &b2) >= sameCat {
		t.Error("same-category boost expected")
	}
	if sameCat < Threshold {
		t.Errorf("related texts should be similar, score=%.2f", sameCat)
	}
	if s := Score(&a, &c); s >= Threshold {
		t.Errorf("unrelated texts scored %.2f", s)
	}
}

func TestClusterFindsGroupsAndIgnoresNoise(t *testing.T) {
	docs := []Doc{
		{ID: 1, CategoryID: 1, Title: "Двойной ввод сделок в Битрикс24 и 1С", Body: "Менеджеры вводят данные клиента в Битрикс24, потом вручную переносят те же данные в 1С. Двойной ввод даёт ошибки в реквизитах."},
		{ID: 2, CategoryID: 1, Title: "Ручной перенос заказов из Битрикс24 в 1С", Body: "Каждый заказ вручную переносим из Битрикс24 в 1С: реквизиты клиента, позиции. Двойной ввод данных и ошибки."},
		{ID: 3, CategoryID: 1, Title: "Данные клиента вводим дважды", Body: "Данные клиента и реквизиты вводим дважды — в Битрикс24 и в 1С, при ручном переносе ошибки."},
		{ID: 4, CategoryID: 2, Title: "Холодно на складе", Body: "Зимой на складе температура около восьми градусов, кладовщики мёрзнут."},
		{ID: 5, CategoryID: 3, Title: "Кофемашина сломалась", Body: "Кофемашина на третьем этаже не работает вторую неделю."},
	}
	groups := Cluster(docs, Threshold, MinGroupSize)
	if len(groups) != 1 || len(groups[0].Members) != 3 {
		t.Fatalf("want one group of 3, got %+v", groups)
	}
	for _, m := range groups[0].Members {
		if docs[m].ID > 3 {
			t.Fatalf("noise doc %d grouped", docs[m].ID)
		}
	}
	if groups[0].Score < Threshold || groups[0].Score > 1 {
		t.Fatalf("score out of range: %v", groups[0].Score)
	}
}
