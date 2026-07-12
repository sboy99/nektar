package digest

import (
	"sort"
	"strings"
	"time"

	"github.com/sboy99/nektar/shared/domain"
)

const articleCountWeight = 10.0

// scoredArticle is an article with an importance score.
type scoredArticle struct {
	Article *domain.Article
	Score   float64
}

// scoredCluster is a cluster with ranked articles and an importance score.
type scoredCluster struct {
	Cluster  *domain.Cluster
	Articles []scoredArticle
	Score    float64
}

func scoreArticle(a *domain.Article, now time.Time) float64 {
	hours := now.Sub(a.CreatedAt).Hours()
	if hours < 0 {
		hours = 0
	}
	recency := 24.0 / (1.0 + hours)
	reading := float64(a.ReadingTimeMinutes)
	if reading <= 0 {
		reading = 1
	}
	return recency + reading
}

func rankAndFilter(
	clusters []*domain.Cluster,
	articlesByCluster map[string][]*domain.Article,
	now time.Time,
	lookback time.Duration,
	minClusterSize int,
	maxClusters int,
	maxArticlesPerCluster int,
) []scoredCluster {
	cutoff := now.Add(-lookback)
	if minClusterSize <= 0 {
		minClusterSize = 1
	}
	if maxClusters <= 0 {
		maxClusters = 10
	}
	if maxArticlesPerCluster <= 0 {
		maxArticlesPerCluster = 5
	}

	var ranked []scoredCluster
	for _, c := range clusters {
		raw := articlesByCluster[c.ID]
		var scored []scoredArticle
		for _, a := range raw {
			if a == nil {
				continue
			}
			if !a.CreatedAt.IsZero() && a.CreatedAt.Before(cutoff) {
				continue
			}
			if !eligibleStage(a.Stage) {
				continue
			}
			scored = append(scored, scoredArticle{
				Article: a,
				Score:   scoreArticle(a, now),
			})
		}
		if len(scored) < minClusterSize {
			continue
		}

		sort.Slice(scored, func(i, j int) bool {
			if scored[i].Score == scored[j].Score {
				return scored[i].Article.CreatedAt.After(scored[j].Article.CreatedAt)
			}
			return scored[i].Score > scored[j].Score
		})
		if len(scored) > maxArticlesPerCluster {
			scored = scored[:maxArticlesPerCluster]
		}

		sum := 0.0
		for _, sa := range scored {
			sum += sa.Score
		}
		clusterScore := float64(len(scored))*articleCountWeight + sum
		ranked = append(ranked, scoredCluster{
			Cluster:  c,
			Articles: scored,
			Score:    clusterScore,
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			return ranked[i].Cluster.ID < ranked[j].Cluster.ID
		}
		return ranked[i].Score > ranked[j].Score
	})
	if len(ranked) > maxClusters {
		ranked = ranked[:maxClusters]
	}
	return ranked
}

func eligibleStage(stage domain.PipelineStage) bool {
	switch stage {
	case domain.StageClustered, domain.StageDigesting, domain.StageDigestReady:
		return true
	default:
		return false
	}
}

func weakClusterName(cluster *domain.Cluster, articles []scoredArticle) bool {
	name := strings.TrimSpace(cluster.Name)
	if name == "" || len(name) < 3 {
		return true
	}
	if len(articles) > 0 && name == articles[0].Article.Title {
		return true
	}
	return false
}

func formatArticlesBlock(articles []scoredArticle) string {
	var b strings.Builder
	for i, sa := range articles {
		a := sa.Article
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("- Title: ")
		b.WriteString(a.Title)
		if a.URL != "" {
			b.WriteString("\n  URL: ")
			b.WriteString(a.URL)
		}
		excerpt := articleExcerpt(a, 400)
		if excerpt != "" {
			b.WriteString("\n  Excerpt: ")
			b.WriteString(excerpt)
		}
	}
	return b.String()
}

func articleExcerpt(a *domain.Article, maxRunes int) string {
	text := strings.TrimSpace(a.PlainText)
	if text == "" {
		text = strings.TrimSpace(a.Markdown)
	}
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return text
}

func estimateTokens(text string) int {
	n := len(text) / 4
	if n < 1 {
		return 1
	}
	return n
}

func readingTimeSum(articles []scoredArticle, wordsPerMinute int) int {
	total := 0
	for _, sa := range articles {
		rt := sa.Article.ReadingTimeMinutes
		if rt <= 0 {
			rt = estimateReadingTime(sa.Article, wordsPerMinute)
		}
		total += rt
	}
	if total < 1 {
		return 1
	}
	return total
}

func estimateReadingTime(a *domain.Article, wordsPerMinute int) int {
	if wordsPerMinute <= 0 {
		wordsPerMinute = 200
	}
	text := strings.TrimSpace(a.PlainText)
	if text == "" {
		text = strings.TrimSpace(a.Markdown)
	}
	words := len(strings.Fields(text))
	if words == 0 {
		return 1
	}
	minutes := (words + wordsPerMinute - 1) / wordsPerMinute
	if minutes < 1 {
		return 1
	}
	return minutes
}
