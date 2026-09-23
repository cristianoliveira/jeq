def square($x): $x * $x;
def complete_probabilities($answer; $criteria):
  (($answer.probabilities | type) == "object") and
  (($answer.probabilities | keys | sort) == ($criteria | keys | sort)) and
  (all($answer.probabilities[]; type == "number" and . >= 0 and . <= 1)) and
  ((([$answer.probabilities[]] | add) - 1 | fabs) <= 0.001);
def base($case; $response; $latency): {
  id:$case.id,
  family:$case.family,
  primitive:$case.primitive,
  latency_ms:$latency,
  model:($response.model // null),
  usage:($response.usage // null)
};

$case[0] as $c | $response[0] as $r | $c.expected as $e |
if (($r.answers.answer // null) | type) != "object" then
  base($c; $r; $latency_ms) + {status:"invalid_response",correct:false,useful:false,in_band:null,brier:null,absolute_error:null,actual:null,expected:$e}
else
  $r.answers.answer as $a |
  if $e.kind == "choice" and ($a.choice | type) == "string" then
    (complete_probabilities($a; $c.question.criteria)) as $probabilities_complete |
    (if $probabilities_complete then
      [($a.probabilities | to_entries[]) | square(.value - (if .key == $e.value then 1 else 0 end))] | add
    else null end) as $brier |
    ($a.choice == $e.value) as $correct |
    base($c; $r; $latency_ms) + {
      status:"ok",actual:$a.choice,correct:$correct,useful:$correct,in_band:null,brier:$brier,
      probability_metric_status:(if $probabilities_complete then "scored" else "unscorable" end),
      absolute_error:null,expected:$e.value
    }
  elif $e.kind == "noul" and ($a.noul | type) == "number" then
    (($a.noul >= 0.5) == ($e.target == 1)) as $correct |
    base($c; $r; $latency_ms) + {
      status:"ok",actual:$a.noul,correct:$correct,useful:$correct,
      in_band:($a.noul >= $e.min and $a.noul <= $e.max),brier:square($a.noul - $e.target),
      probability_metric_status:"scored",absolute_error:null,expected:$e.target
    }
  elif $e.kind == "score" and ($a.score | type) == "number" then
    ($a.score >= $e.min and $a.score <= $e.max) as $correct |
    base($c; $r; $latency_ms) + {
      status:"ok",actual:$a.score,correct:$correct,useful:$correct,in_band:$correct,brier:null,
      probability_metric_status:null,absolute_error:(($a.score - $e.point) | fabs),expected:$e.point
    }
  elif $e.kind == "rank" and complete_probabilities($a; $c.question.criteria) then
    ([$a.probabilities | to_entries | sort_by([-(.value), .key])[] | .key]) as $order |
    ([$a.probabilities | to_entries[] | square(.value - (if .key == $e.order[0] then 1 else 0 end))] | add) as $brier |
    ($order == $e.order) as $correct | ($order[0] == $e.order[0]) as $top1 |
    base($c; $r; $latency_ms) + {
      status:"ok",actual:$order,correct:$correct,useful:$top1,top1_correct:$top1,in_band:null,brier:$brier,
      probability_metric_status:"scored",absolute_error:null,expected:$e.order
    }
  else
    base($c; $r; $latency_ms) + {status:"unscorable",correct:false,useful:false,in_band:null,brier:null,absolute_error:null,actual:null,expected:$e}
  end
end
